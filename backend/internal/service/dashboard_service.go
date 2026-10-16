package service

import (
	"context"
	"encoding/json"

	"muara.bi/core/internal/domain/entity"
	"muara.bi/core/internal/dto/request"
	"muara.bi/core/internal/dto/response"
	"muara.bi/core/internal/repository/pgsql"
	"muara.bi/core/pkg/customerror"
)

const defaultCacheTTLSeconds = 300

type DashboardServiceProvider interface {
	Create(ctx context.Context, wsID, projID, userID string, req request.CreateDashboardRequest) (*response.DashboardSummaryResponse, error)
	List(ctx context.Context, wsID, projID string) ([]response.DashboardSummaryResponse, error)
	Get(ctx context.Context, wsID, dashID string) (*response.DashboardDetailResponse, error)
	Update(ctx context.Context, wsID, dashID string, req request.UpdateDashboardRequest) (*response.DashboardSummaryResponse, error)
}

type DashboardService struct {
	dashRepo    pgsql.DashboardRepositoryProvider
	widgetRepo  pgsql.WidgetRepositoryProvider
	projectRepo pgsql.ProjectRepositoryProvider
}

func NewDashboardService(
	dashRepo pgsql.DashboardRepositoryProvider,
	widgetRepo pgsql.WidgetRepositoryProvider,
	projectRepo pgsql.ProjectRepositoryProvider,
) *DashboardService {
	return &DashboardService{
		dashRepo:    dashRepo,
		widgetRepo:  widgetRepo,
		projectRepo: projectRepo,
	}
}

func (s *DashboardService) Create(ctx context.Context, wsID, projID, userID string, req request.CreateDashboardRequest) (*response.DashboardSummaryResponse, error) {
	project, err := s.projectRepo.GetProjectByID(ctx, wsID, projID)
	if err != nil {
		return nil, customerror.NewInternalError("failed to load project", err)
	}
	if project == nil {
		return nil, customerror.NewNotFoundError("project not found", nil)
	}
	if req.Title == "" {
		return nil, customerror.NewBadRequestError("dashboard title is required", nil)
	}

	dashboard := &entity.Dashboard{
		WorkspaceID:     wsID,
		ProjectID:       projID,
		Title:           req.Title,
		Description:     req.Description,
		LayoutConfig:    req.LayoutConfig,
		GlobalFilters:   req.GlobalFilters,
		CacheTTLSeconds: resolveTTL(req.CacheTTLSeconds),
		CreatedBy:       &userID,
	}
	if err := s.dashRepo.CreateDashboard(ctx, dashboard); err != nil {
		return nil, customerror.NewInternalError("failed to create dashboard", err)
	}

	return dashboardSummary(dashboard), nil
}

func (s *DashboardService) List(ctx context.Context, wsID, projID string) ([]response.DashboardSummaryResponse, error) {
	dashboards, err := s.dashRepo.ListDashboardsByProject(ctx, wsID, projID)
	if err != nil {
		return nil, customerror.NewInternalError("failed to list dashboards", err)
	}

	results := make([]response.DashboardSummaryResponse, 0, len(dashboards))
	for i := range dashboards {
		results = append(results, *dashboardSummary(&dashboards[i]))
	}
	return results, nil
}

func (s *DashboardService) Get(ctx context.Context, wsID, dashID string) (*response.DashboardDetailResponse, error) {
	dashboard, err := s.dashRepo.GetDashboardByID(ctx, wsID, dashID)
	if err != nil {
		return nil, customerror.NewInternalError("failed to load dashboard", err)
	}
	if dashboard == nil {
		return nil, customerror.NewNotFoundError("dashboard not found", nil)
	}

	widgets, err := s.widgetRepo.ListWidgetsByDashboard(ctx, dashID)
	if err != nil {
		return nil, customerror.NewInternalError("failed to load dashboard widgets", err)
	}
	dashboard.Widgets = widgets

	return dashboardDetail(dashboard), nil
}

// Update persists the debounced canvas auto-save payload. Layout and filters are
// replaced wholesale because the canvas is the single source of truth.
func (s *DashboardService) Update(ctx context.Context, wsID, dashID string, req request.UpdateDashboardRequest) (*response.DashboardSummaryResponse, error) {
	existing, err := s.dashRepo.GetDashboardByID(ctx, wsID, dashID)
	if err != nil {
		return nil, customerror.NewInternalError("failed to load dashboard", err)
	}
	if existing == nil {
		return nil, customerror.NewNotFoundError("dashboard not found", nil)
	}

	if req.Title != "" {
		existing.Title = req.Title
	}
	existing.Description = req.Description
	if len(req.LayoutConfig) > 0 {
		existing.LayoutConfig = req.LayoutConfig
	}
	if len(req.GlobalFilters) > 0 {
		existing.GlobalFilters = req.GlobalFilters
	}
	existing.CacheTTLSeconds = resolveTTL(req.CacheTTLSeconds)

	if err := s.dashRepo.UpdateDashboard(ctx, existing); err != nil {
		return nil, customerror.NewInternalError("failed to update dashboard", err)
	}
	return dashboardSummary(existing), nil
}

func resolveTTL(ttl *int) int {
	if ttl == nil {
		return defaultCacheTTLSeconds
	}
	if *ttl < 0 {
		return 0
	}
	return *ttl
}

func dashboardSummary(d *entity.Dashboard) *response.DashboardSummaryResponse {
	return &response.DashboardSummaryResponse{
		ID:              d.ID,
		WorkspaceID:     d.WorkspaceID,
		ProjectID:       d.ProjectID,
		Title:           d.Title,
		Description:     d.Description,
		LayoutConfig:    jsonOrArray(d.LayoutConfig),
		GlobalFilters:   jsonOrArray(d.GlobalFilters),
		CacheTTLSeconds: d.CacheTTLSeconds,
		CreatedBy:       d.CreatedBy,
		UpdatedAt:       d.UpdatedAt,
	}
}

func dashboardDetail(d *entity.Dashboard) *response.DashboardDetailResponse {
	widgets := make([]response.WidgetResponse, 0, len(d.Widgets))
	for i := range d.Widgets {
		widgets = append(widgets, *widgetResponse(&d.Widgets[i]))
	}

	return &response.DashboardDetailResponse{
		DashboardSummaryResponse: *dashboardSummary(d),
		Widgets:                  widgets,
	}
}

func jsonOrArray(raw json.RawMessage) json.RawMessage {
	if len(raw) == 0 {
		return json.RawMessage("[]")
	}
	return raw
}
