package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/hibiken/asynq"
	"github.com/robfig/cron/v3"
	"muara.bi/core/internal/domain/entity"
	"muara.bi/core/internal/dto/request"
	"muara.bi/core/internal/dto/response"
	"muara.bi/core/internal/repository/pgsql"
	"muara.bi/core/internal/tasks"
	"muara.bi/core/pkg/customerror"
	"muara.bi/core/pkg/logging"
)

var cronParser = cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow | cron.Descriptor)

type AlertServiceProvider interface {
	Create(ctx context.Context, wsID, dashID, userID string, req request.CreateAlertRequest) (*response.AlertResponse, error)
	ListByDashboard(ctx context.Context, wsID, dashID string) ([]response.AlertResponse, error)
	GetByID(ctx context.Context, wsID, alertID string) (*response.AlertResponse, error)
	Update(ctx context.Context, wsID, alertID string, req request.UpdateAlertRequest) (*response.AlertResponse, error)
	Delete(ctx context.Context, wsID, alertID string) error
	TestAlert(ctx context.Context, wsID, alertID string) error
}

type AlertService struct {
	alertRepo     pgsql.AlertRepositoryProvider
	dashboardRepo pgsql.DashboardRepositoryProvider
	widgetRepo    pgsql.WidgetRepositoryProvider
	asynqClient   *asynq.Client
	logger        *logging.Logger
}

func NewAlertService(
	alertRepo pgsql.AlertRepositoryProvider,
	dashboardRepo pgsql.DashboardRepositoryProvider,
	widgetRepo pgsql.WidgetRepositoryProvider,
	asynqClient *asynq.Client,
	logger *logging.Logger,
) *AlertService {
	return &AlertService{
		alertRepo:     alertRepo,
		dashboardRepo: dashboardRepo,
		widgetRepo:    widgetRepo,
		asynqClient:   asynqClient,
		logger:        logger,
	}
}

func (s *AlertService) Create(ctx context.Context, wsID, dashID, userID string, req request.CreateAlertRequest) (*response.AlertResponse, error) {
	// 1. Verify dashboard belongs to workspace
	dash, err := s.dashboardRepo.GetDashboardByID(ctx, wsID, dashID)
	if err != nil {
		return nil, customerror.NewInternalError("failed to verify dashboard", err)
	}
	if dash == nil {
		return nil, customerror.NewNotFoundError("dashboard not found", nil)
	}

	title := strings.TrimSpace(req.Title)
	if title == "" {
		return nil, customerror.NewBadRequestError("title is required", nil)
	}

	alertType := strings.ToUpper(strings.TrimSpace(req.AlertType))
	if alertType != entity.AlertTypeScheduledDigest && alertType != entity.AlertTypeThresholdAlert {
		return nil, customerror.NewBadRequestError("alert_type must be SCHEDULED_DIGEST or THRESHOLD_ALERT", nil)
	}

	// 2. Validate cron expression
	cronExpr := strings.TrimSpace(req.CronExpression)
	if _, err := cronParser.Parse(cronExpr); err != nil {
		return nil, customerror.NewBadRequestError(fmt.Sprintf("invalid cron expression %q: %v", cronExpr, err), nil)
	}

	// 3. Validate channel
	channel := strings.ToUpper(strings.TrimSpace(req.Channel))
	switch channel {
	case entity.ChannelTelegram, entity.ChannelSlack, entity.ChannelEmail, entity.ChannelWebhook:
	default:
		return nil, customerror.NewBadRequestError("channel must be TELEGRAM, SLACK, EMAIL, or WEBHOOK", nil)
	}

	// 4. Validate threshold requirements if applicable
	if alertType == entity.AlertTypeThresholdAlert {
		if req.WidgetID == nil || strings.TrimSpace(*req.WidgetID) == "" {
			return nil, customerror.NewBadRequestError("widget_id is required for THRESHOLD_ALERT", nil)
		}
		wID := strings.TrimSpace(*req.WidgetID)
		widget, err := s.widgetRepo.GetWidgetByID(ctx, wsID, wID)
		if err != nil {
			return nil, customerror.NewInternalError("failed to verify widget", err)
		}
		if widget == nil || widget.DashboardID != dashID {
			return nil, customerror.NewBadRequestError("widget does not belong to this dashboard", nil)
		}

		if req.ConditionOperator == nil {
			return nil, customerror.NewBadRequestError("condition_operator is required for THRESHOLD_ALERT", nil)
		}
		op := strings.TrimSpace(*req.ConditionOperator)
		switch op {
		case ">", "<", ">=", "<=", "==", "!=":
		default:
			return nil, customerror.NewBadRequestError("condition_operator must be >, <, >=, <=, ==, or !=", nil)
		}

		if req.ThresholdValue == nil {
			return nil, customerror.NewBadRequestError("threshold_value is required for THRESHOLD_ALERT", nil)
		}
	}

	// Recipient config validation
	recipientBytes := req.RecipientConfig
	if len(recipientBytes) == 0 {
		recipientBytes = json.RawMessage("{}")
	}

	isActive := true
	if req.IsActive != nil {
		isActive = *req.IsActive
	}

	alert := &entity.DashboardAlert{
		WorkspaceID:       wsID,
		DashboardID:       dashID,
		WidgetID:          req.WidgetID,
		Title:             title,
		AlertType:         alertType,
		CronExpression:    cronExpr,
		ConditionOperator: req.ConditionOperator,
		ThresholdValue:    req.ThresholdValue,
		Channel:           channel,
		RecipientConfig:   recipientBytes,
		EnvTag:            NormalizeEnvTag(req.EnvTag),
		IsActive:          isActive,
		CreatedBy:         nilIfEmpty(userID),
	}

	if err := s.alertRepo.CreateAlert(ctx, alert); err != nil {
		return nil, customerror.NewInternalError("failed to create alert", err)
	}

	return toAlertResponse(alert), nil
}

func (s *AlertService) ListByDashboard(ctx context.Context, wsID, dashID string) ([]response.AlertResponse, error) {
	dash, err := s.dashboardRepo.GetDashboardByID(ctx, wsID, dashID)
	if err != nil {
		return nil, customerror.NewInternalError("failed to verify dashboard", err)
	}
	if dash == nil {
		return nil, customerror.NewNotFoundError("dashboard not found", nil)
	}

	alerts, err := s.alertRepo.ListAlertsByDashboard(ctx, wsID, dashID)
	if err != nil {
		return nil, customerror.NewInternalError("failed to list alerts", err)
	}

	resp := make([]response.AlertResponse, len(alerts))
	for i := range alerts {
		resp[i] = *toAlertResponse(&alerts[i])
	}
	return resp, nil
}

func (s *AlertService) GetByID(ctx context.Context, wsID, alertID string) (*response.AlertResponse, error) {
	alert, err := s.alertRepo.GetAlertByID(ctx, wsID, alertID)
	if err != nil {
		return nil, customerror.NewInternalError("failed to get alert", err)
	}
	if alert == nil {
		return nil, customerror.NewNotFoundError("alert not found", nil)
	}

	return toAlertResponse(alert), nil
}

func (s *AlertService) Update(ctx context.Context, wsID, alertID string, req request.UpdateAlertRequest) (*response.AlertResponse, error) {
	existing, err := s.alertRepo.GetAlertByID(ctx, wsID, alertID)
	if err != nil {
		return nil, customerror.NewInternalError("failed to get alert", err)
	}
	if existing == nil {
		return nil, customerror.NewNotFoundError("alert not found", nil)
	}

	title := strings.TrimSpace(req.Title)
	if title != "" {
		existing.Title = title
	}

	cronExpr := strings.TrimSpace(req.CronExpression)
	if cronExpr != "" {
		if _, err := cronParser.Parse(cronExpr); err != nil {
			return nil, customerror.NewBadRequestError(fmt.Sprintf("invalid cron expression %q: %v", cronExpr, err), nil)
		}
		existing.CronExpression = cronExpr
	}

	channel := strings.ToUpper(strings.TrimSpace(req.Channel))
	if channel != "" {
		switch channel {
		case entity.ChannelTelegram, entity.ChannelSlack, entity.ChannelEmail, entity.ChannelWebhook:
			existing.Channel = channel
		default:
			return nil, customerror.NewBadRequestError("channel must be TELEGRAM, SLACK, EMAIL, or WEBHOOK", nil)
		}
	}

	if req.WidgetID != nil {
		existing.WidgetID = req.WidgetID
	}
	if req.ConditionOperator != nil {
		existing.ConditionOperator = req.ConditionOperator
	}
	if req.ThresholdValue != nil {
		existing.ThresholdValue = req.ThresholdValue
	}
	if len(req.RecipientConfig) > 0 {
		existing.RecipientConfig = req.RecipientConfig
	}
	if req.EnvTag != "" {
		existing.EnvTag = NormalizeEnvTag(req.EnvTag)
	}
	existing.IsActive = req.IsActive

	if err := s.alertRepo.UpdateAlert(ctx, existing); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, customerror.NewNotFoundError("alert not found", nil)
		}
		return nil, customerror.NewInternalError("failed to update alert", err)
	}

	return toAlertResponse(existing), nil
}

func (s *AlertService) Delete(ctx context.Context, wsID, alertID string) error {
	if err := s.alertRepo.DeleteAlert(ctx, wsID, alertID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return customerror.NewNotFoundError("alert not found", nil)
		}
		return customerror.NewInternalError("failed to delete alert", err)
	}
	return nil
}

func (s *AlertService) TestAlert(ctx context.Context, wsID, alertID string) error {
	alert, err := s.alertRepo.GetAlertByID(ctx, wsID, alertID)
	if err != nil {
		return customerror.NewInternalError("failed to get alert", err)
	}
	if alert == nil {
		return customerror.NewNotFoundError("alert not found", nil)
	}

	if s.asynqClient == nil {
		return customerror.NewInternalError("asynq client is not available to dispatch test alert", nil)
	}

	projID, _ := s.dashboardRepo.GetProjectIDByDashboard(ctx, wsID, alert.DashboardID)

	if alert.AlertType == entity.AlertTypeThresholdAlert {
		rawSQL := "SELECT 1 as val"
		if alert.WidgetID != nil {
			widget, err := s.widgetRepo.GetWidgetByID(ctx, wsID, *alert.WidgetID)
			if err == nil && widget != nil {
				rawSQL = widget.RawSQL
			}
		}

		op := ">"
		if alert.ConditionOperator != nil {
			op = *alert.ConditionOperator
		}
		threshold := 0.0
		if alert.ThresholdValue != nil {
			threshold = *alert.ThresholdValue
		}

		task, err := tasks.NewAlertCheckTask(tasks.AlertCheckPayload{
			AlertID:        alert.ID,
			WorkspaceID:    alert.WorkspaceID,
			ProjectID:      projID,
			DashboardID:    alert.DashboardID,
			WidgetID:       safeString(alert.WidgetID),
			Title:          "[TEST] " + alert.Title,
			RawSQL:         rawSQL,
			Operator:       op,
			ThresholdValue: threshold,
			Channel:        alert.Channel,
			RecipientJSON:  string(alert.RecipientConfig),
			EnvTag:         alert.EnvTag,
		})
		if err != nil {
			return customerror.NewInternalError("failed to create test alert task", err)
		}
		_, err = s.asynqClient.EnqueueContext(ctx, task)
		return err
	}

	// SCHEDULED_DIGEST test
	task, err := tasks.NewDigestDispatchTask(tasks.DigestDispatchPayload{
		AlertID:       alert.ID,
		WorkspaceID:   alert.WorkspaceID,
		ProjectID:     projID,
		DashboardID:   alert.DashboardID,
		Title:         "[TEST] " + alert.Title,
		Channel:       alert.Channel,
		RecipientJSON: string(alert.RecipientConfig),
		EnvTag:        alert.EnvTag,
	})
	if err != nil {
		return customerror.NewInternalError("failed to create test digest task", err)
	}
	_, err = s.asynqClient.EnqueueContext(ctx, task)
	return err
}

func toAlertResponse(a *entity.DashboardAlert) *response.AlertResponse {
	return &response.AlertResponse{
		ID:                a.ID,
		WorkspaceID:       a.WorkspaceID,
		DashboardID:       a.DashboardID,
		WidgetID:          a.WidgetID,
		Title:             a.Title,
		AlertType:         a.AlertType,
		CronExpression:    a.CronExpression,
		ConditionOperator: a.ConditionOperator,
		ThresholdValue:    a.ThresholdValue,
		Channel:           a.Channel,
		RecipientConfig:   a.RecipientConfig,
		EnvTag:            a.EnvTag,
		IsActive:          a.IsActive,
		LastTriggeredAt:   a.LastTriggeredAt,
		CreatedAt:         a.CreatedAt,
	}
}

func safeString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
