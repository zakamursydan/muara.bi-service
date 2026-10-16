package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"

	"muara.bi/core/internal/domain/entity"
	"muara.bi/core/internal/dto/request"
	"muara.bi/core/internal/dto/response"
	"muara.bi/core/internal/duckdb"
	"muara.bi/core/internal/repository/pgsql"
	"muara.bi/core/pkg/customerror"
)

const (
	defaultWidgetMaxRows = 10000
	maxWidgetMaxRows     = 100000
)

var chartTypes = map[string]struct{}{
	"line": {}, "bar": {}, "pie": {}, "kpi": {}, "table": {}, "scatter": {}, "area": {},
}

type WidgetServiceProvider interface {
	Create(ctx context.Context, wsID, dashID string, req request.CreateWidgetRequest) (*response.WidgetResponse, error)
	Get(ctx context.Context, wsID, widgetID string) (*response.WidgetResponse, error)
	Update(ctx context.Context, wsID, widgetID string, req request.UpdateWidgetRequest) (*response.WidgetResponse, error)
	Delete(ctx context.Context, wsID, widgetID string) error
}

type WidgetService struct {
	widgetRepo pgsql.WidgetRepositoryProvider
	dashRepo   pgsql.DashboardRepositoryProvider
}

func NewWidgetService(
	widgetRepo pgsql.WidgetRepositoryProvider,
	dashRepo pgsql.DashboardRepositoryProvider,
) *WidgetService {
	return &WidgetService{widgetRepo: widgetRepo, dashRepo: dashRepo}
}

func (s *WidgetService) Create(ctx context.Context, wsID, dashID string, req request.CreateWidgetRequest) (*response.WidgetResponse, error) {
	dashboard, err := s.dashRepo.GetDashboardByID(ctx, wsID, dashID)
	if err != nil {
		return nil, customerror.NewInternalError("failed to load dashboard", err)
	}
	if dashboard == nil {
		return nil, customerror.NewNotFoundError("dashboard not found", nil)
	}

	widget, err := buildWidget(req.QueryMode, req.ChartType, req.Title, req.BuilderState, req.RawSQL, req.EchartsConfig, req.CacheTTLSeconds, req.MaxRows)
	if err != nil {
		return nil, err
	}
	widget.DashboardID = dashID

	if err := s.widgetRepo.CreateWidget(ctx, widget); err != nil {
		return nil, customerror.NewInternalError("failed to create widget", err)
	}
	return widgetResponse(widget), nil
}

func (s *WidgetService) Get(ctx context.Context, wsID, widgetID string) (*response.WidgetResponse, error) {
	widget, err := s.widgetRepo.GetWidgetByID(ctx, wsID, widgetID)
	if err != nil {
		return nil, customerror.NewInternalError("failed to load widget", err)
	}
	if widget == nil {
		return nil, customerror.NewNotFoundError("widget not found", nil)
	}
	return widgetResponse(widget), nil
}

func (s *WidgetService) Update(ctx context.Context, wsID, widgetID string, req request.UpdateWidgetRequest) (*response.WidgetResponse, error) {
	existing, err := s.widgetRepo.GetWidgetByID(ctx, wsID, widgetID)
	if err != nil {
		return nil, customerror.NewInternalError("failed to load widget", err)
	}
	if existing == nil {
		return nil, customerror.NewNotFoundError("widget not found", nil)
	}

	updated, err := buildWidget(req.QueryMode, req.ChartType, req.Title, req.BuilderState, req.RawSQL, req.EchartsConfig, req.CacheTTLSeconds, req.MaxRows)
	if err != nil {
		return nil, err
	}
	if len(req.EchartsConfig) == 0 {
		updated.EchartsConfig = existing.EchartsConfig
	}
	updated.ID = existing.ID
	updated.DashboardID = existing.DashboardID
	updated.CreatedAt = existing.CreatedAt

	if err := s.widgetRepo.UpdateWidget(ctx, wsID, updated); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, customerror.NewNotFoundError("widget not found", nil)
		}
		return nil, customerror.NewInternalError("failed to update widget", err)
	}
	return widgetResponse(updated), nil
}

func (s *WidgetService) Delete(ctx context.Context, wsID, widgetID string) error {
	if err := s.widgetRepo.DeleteWidget(ctx, wsID, widgetID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return customerror.NewNotFoundError("widget not found", nil)
		}
		return customerror.NewInternalError("failed to delete widget", err)
	}
	return nil
}

func buildWidget(
	queryMode, chartType, title string,
	builderState json.RawMessage,
	rawSQL string,
	echartsConfig json.RawMessage,
	cacheTTL, maxRows *int,
) (*entity.Widget, error) {
	if strings.TrimSpace(title) == "" {
		return nil, customerror.NewBadRequestError("widget title is required", nil)
	}

	compiledSQL, mode, err := resolveWidgetSQL(queryMode, builderState, rawSQL)
	if err != nil {
		return nil, err
	}
	normalizedChart, err := normalizeChartType(chartType)
	if err != nil {
		return nil, err
	}

	return &entity.Widget{
		Title:           title,
		ChartType:       normalizedChart,
		QueryMode:       mode,
		BuilderState:    builderState,
		RawSQL:          compiledSQL,
		EchartsConfig:   echartsConfig,
		CacheTTLSeconds: resolveTTL(cacheTTL),
		MaxRows:         resolveMaxRows(maxRows),
	}, nil
}

// resolveWidgetSQL enforces the SYNCED / CUSTOM_SQL contract. In SYNCED mode the
// stored query is always recompiled from the visual builder state, which makes
// it impossible to smuggle hand-written SQL past the no-code editor.
func resolveWidgetSQL(queryMode string, builderState json.RawMessage, rawSQL string) (string, string, error) {
	mode := strings.ToUpper(strings.TrimSpace(queryMode))
	if mode == "" {
		mode = entity.QueryModeSynced
	}

	switch mode {
	case entity.QueryModeSynced:
		if len(builderState) == 0 {
			return "", "", customerror.NewBadRequestError("builder_state is required for SYNCED widgets", nil)
		}
		var state entity.WidgetBuilderState
		if err := json.Unmarshal(builderState, &state); err != nil {
			return "", "", customerror.NewBadRequestError("invalid builder_state payload", err)
		}
		compiled, err := CompileBuilderSQL(state)
		if err != nil {
			return "", "", customerror.NewBadRequestError("failed to compile visual builder query", err)
		}
		if err := duckdb.ValidateQuery(compiled); err != nil {
			return "", "", customerror.NewBadRequestError("compiled query rejected by guardrail", err)
		}
		return compiled, mode, nil

	case entity.QueryModeCustomSQL:
		if strings.TrimSpace(rawSQL) == "" {
			return "", "", customerror.NewBadRequestError("raw_sql is required for CUSTOM_SQL widgets", nil)
		}
		if err := duckdb.ValidateQuery(rawSQL); err != nil {
			return "", "", customerror.NewBadRequestError("query guardrail violation", err)
		}
		return rawSQL, mode, nil

	default:
		return "", "", customerror.NewBadRequestError("query_mode must be SYNCED or CUSTOM_SQL", nil)
	}
}

func normalizeChartType(chartType string) (string, error) {
	normalized := strings.ToLower(strings.TrimSpace(chartType))
	if _, ok := chartTypes[normalized]; !ok {
		return "", customerror.NewBadRequestError("unsupported chart_type: "+chartType, nil)
	}
	return normalized, nil
}

func resolveMaxRows(maxRows *int) int {
	if maxRows == nil || *maxRows <= 0 {
		return defaultWidgetMaxRows
	}
	if *maxRows > maxWidgetMaxRows {
		return maxWidgetMaxRows
	}
	return *maxRows
}

func widgetResponse(w *entity.Widget) *response.WidgetResponse {
	return &response.WidgetResponse{
		ID:              w.ID,
		DashboardID:     w.DashboardID,
		Title:           w.Title,
		ChartType:       w.ChartType,
		QueryMode:       w.QueryMode,
		BuilderState:    w.BuilderState,
		RawSQL:          w.RawSQL,
		EchartsConfig:   jsonOrObject(w.EchartsConfig),
		CacheTTLSeconds: w.CacheTTLSeconds,
		MaxRows:         w.MaxRows,
		CreatedAt:       w.CreatedAt,
	}
}

func jsonOrObject(raw json.RawMessage) json.RawMessage {
	if len(raw) == 0 {
		return json.RawMessage("{}")
	}
	return raw
}
