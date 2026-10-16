package worker

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/hibiken/asynq"
	"github.com/robfig/cron/v3"
	"muara.bi/core/internal/domain/entity"
	"muara.bi/core/internal/repository/pgsql"
	"muara.bi/core/pkg/logging"
)

// Scheduler periodically evaluates active alerts against their cron expressions
type Scheduler struct {
	alertRepo     pgsql.AlertRepositoryProvider
	dashboardRepo pgsql.DashboardRepositoryProvider
	widgetRepo    pgsql.WidgetRepositoryProvider
	asynqClient   *asynq.Client
	logger        *logging.Logger
	cronParser    cron.Parser
}

func NewScheduler(
	alertRepo pgsql.AlertRepositoryProvider,
	dashboardRepo pgsql.DashboardRepositoryProvider,
	widgetRepo pgsql.WidgetRepositoryProvider,
	asynqClient *asynq.Client,
	logger *logging.Logger,
) *Scheduler {
	return &Scheduler{
		alertRepo:     alertRepo,
		dashboardRepo: dashboardRepo,
		widgetRepo:    widgetRepo,
		asynqClient:   asynqClient,
		logger:        logger,
		cronParser:    cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow | cron.Descriptor),
	}
}

// Start runs the periodic scanner in a background goroutine
func (s *Scheduler) Start(ctx context.Context) {
	s.logger.Info("starting asynq alert & digest cron scheduler (interval: 1m)")

	go func() {
		ticker := time.NewTicker(1 * time.Minute)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				s.logger.Info("stopping alert & digest scheduler")
				return
			case tickTime := <-ticker.C:
				s.scanAndDispatch(ctx, tickTime)
			}
		}
	}()
}

func (s *Scheduler) scanAndDispatch(ctx context.Context, now time.Time) {
	alerts, err := s.alertRepo.ListActiveAlerts(ctx)
	if err != nil {
		s.logger.Error(fmt.Sprintf("scheduler: failed to list active alerts: %v", err))
		return
	}

	for _, alert := range alerts {
		schedule, err := s.cronParser.Parse(alert.CronExpression)
		if err != nil {
			s.logger.Warn(fmt.Sprintf("scheduler: invalid cron %q for alert %s: %v", alert.CronExpression, alert.ID, err))
			continue
		}

		// Check if already triggered recently (within 50s)
		if alert.LastTriggeredAt != nil && now.Sub(*alert.LastTriggeredAt) < 50*time.Second {
			continue
		}

		// Calculate next activation from 1 minute ago to see if it should fire now
		prevMinute := now.Add(-1 * time.Minute)
		nextTime := schedule.Next(prevMinute)

		// If nextTime falls in current window [prevMinute, now]
		if nextTime.Before(now) || nextTime.Equal(now) {
			s.dispatchAlert(ctx, alert)
		}
	}
}

func (s *Scheduler) dispatchAlert(ctx context.Context, alert entity.DashboardAlert) {
	projID, _ := s.dashboardRepo.GetProjectIDByDashboard(ctx, alert.WorkspaceID, alert.DashboardID)

	switch alert.AlertType {
	case entity.AlertTypeThresholdAlert:
		rawSQL := "SELECT 1"
		if alert.WidgetID != nil {
			widget, err := s.widgetRepo.GetWidgetByID(ctx, alert.WorkspaceID, *alert.WidgetID)
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

		widgetIDStr := ""
		if alert.WidgetID != nil {
			widgetIDStr = *alert.WidgetID
		}

		task, err := NewAlertCheckTask(AlertCheckPayload{
			AlertID:        alert.ID,
			WorkspaceID:    alert.WorkspaceID,
			ProjectID:      projID,
			DashboardID:    alert.DashboardID,
			WidgetID:       widgetIDStr,
			Title:          alert.Title,
			RawSQL:         rawSQL,
			Operator:       op,
			ThresholdValue: threshold,
			Channel:        alert.Channel,
			RecipientJSON:  string(alert.RecipientConfig),
			EnvTag:         alert.EnvTag,
		})
		if err != nil {
			s.logger.Error(fmt.Sprintf("scheduler: failed to create alert task: %v", err))
			return
		}

		if _, err := s.asynqClient.EnqueueContext(ctx, task); err != nil {
			s.logger.Error(fmt.Sprintf("scheduler: failed to enqueue alert task %s: %v", alert.ID, err))
			return
		}
		_ = s.alertRepo.UpdateLastTriggeredAt(ctx, alert.ID)
		s.logger.Info(fmt.Sprintf("scheduler: triggered threshold alert %s (%s)", alert.ID, alert.Title))

	case entity.AlertTypeScheduledDigest:
		task, err := NewDigestDispatchTask(DigestDispatchPayload{
			AlertID:       alert.ID,
			WorkspaceID:   alert.WorkspaceID,
			ProjectID:     projID,
			DashboardID:   alert.DashboardID,
			Title:         alert.Title,
			Channel:       alert.Channel,
			RecipientJSON: string(alert.RecipientConfig),
			EnvTag:        alert.EnvTag,
		})
		if err != nil {
			s.logger.Error(fmt.Sprintf("scheduler: failed to create digest task: %v", err))
			return
		}

		if _, err := s.asynqClient.EnqueueContext(ctx, task); err != nil {
			s.logger.Error(fmt.Sprintf("scheduler: failed to enqueue digest task %s: %v", alert.ID, err))
			return
		}
		_ = s.alertRepo.UpdateLastTriggeredAt(ctx, alert.ID)
		s.logger.Info(fmt.Sprintf("scheduler: triggered scheduled digest %s (%s)", alert.ID, alert.Title))
	}
}

// NormalizeEnvTag strips whitespace and defaults empty tags
func NormalizeEnvTag(tag string) string {
	clean := strings.TrimSpace(tag)
	if clean == "" {
		return "default"
	}
	return clean
}
