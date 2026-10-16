package service

import (
	"context"
	"testing"

	"muara.bi/core/internal/domain/entity"
	"muara.bi/core/internal/dto/request"
	"muara.bi/core/pkg/customerror"
)

type mockDashboardRepo struct {
	dashboards map[string]*entity.Dashboard
}

func (m *mockDashboardRepo) CreateDashboard(ctx context.Context, d *entity.Dashboard) error {
	return nil
}
func (m *mockDashboardRepo) GetDashboardByID(ctx context.Context, wsID, id string) (*entity.Dashboard, error) {
	if d, ok := m.dashboards[id]; ok && d.WorkspaceID == wsID {
		return d, nil
	}
	return nil, nil
}
func (m *mockDashboardRepo) ListDashboardsByProject(ctx context.Context, wsID, projID string) ([]entity.Dashboard, error) {
	return nil, nil
}
func (m *mockDashboardRepo) UpdateDashboard(ctx context.Context, d *entity.Dashboard) error {
	return nil
}
func (m *mockDashboardRepo) DeleteDashboard(ctx context.Context, wsID, id string) error {
	return nil
}
func (m *mockDashboardRepo) GetWorkspaceIDByDashboard(ctx context.Context, dashboardID string) (string, error) {
	if d, ok := m.dashboards[dashboardID]; ok {
		return d.WorkspaceID, nil
	}
	return "", nil
}
func (m *mockDashboardRepo) GetProjectIDByDashboard(ctx context.Context, wsID, dashID string) (string, error) {
	if d, ok := m.dashboards[dashID]; ok && d.WorkspaceID == wsID {
		return d.ProjectID, nil
	}
	return "", nil
}

type mockWidgetRepo struct {
	widgets map[string]*entity.Widget
}

func (m *mockWidgetRepo) CreateWidget(ctx context.Context, w *entity.Widget) error { return nil }
func (m *mockWidgetRepo) GetWidgetByID(ctx context.Context, wsID, id string) (*entity.Widget, error) {
	if w, ok := m.widgets[id]; ok {
		return w, nil
	}
	return nil, nil
}
func (m *mockWidgetRepo) ListWidgetsByDashboard(ctx context.Context, dashID string) ([]entity.Widget, error) {
	return nil, nil
}
func (m *mockWidgetRepo) UpdateWidget(ctx context.Context, wsID string, w *entity.Widget) error {
	return nil
}
func (m *mockWidgetRepo) DeleteWidget(ctx context.Context, wsID, id string) error  { return nil }
func (m *mockWidgetRepo) GetProjectIDByWidget(ctx context.Context, wsID, widgetID string) (string, error) {
	return "", nil
}

type mockAlertRepo struct {
	alerts map[string]*entity.DashboardAlert
}

func (m *mockAlertRepo) CreateAlert(ctx context.Context, a *entity.DashboardAlert) error {
	a.ID = "alert-123"
	m.alerts[a.ID] = a
	return nil
}
func (m *mockAlertRepo) GetAlertByID(ctx context.Context, wsID, id string) (*entity.DashboardAlert, error) {
	if a, ok := m.alerts[id]; ok && a.WorkspaceID == wsID {
		return a, nil
	}
	return nil, nil
}
func (m *mockAlertRepo) ListAlertsByDashboard(ctx context.Context, wsID, dashID string) ([]entity.DashboardAlert, error) {
	var list []entity.DashboardAlert
	for _, a := range m.alerts {
		if a.WorkspaceID == wsID && a.DashboardID == dashID {
			list = append(list, *a)
		}
	}
	return list, nil
}
func (m *mockAlertRepo) ListActiveAlerts(ctx context.Context) ([]entity.DashboardAlert, error) {
	var list []entity.DashboardAlert
	for _, a := range m.alerts {
		if a.IsActive {
			list = append(list, *a)
		}
	}
	return list, nil
}
func (m *mockAlertRepo) UpdateAlert(ctx context.Context, a *entity.DashboardAlert) error {
	m.alerts[a.ID] = a
	return nil
}
func (m *mockAlertRepo) DeleteAlert(ctx context.Context, wsID, id string) error {
	delete(m.alerts, id)
	return nil
}
func (m *mockAlertRepo) UpdateLastTriggeredAt(ctx context.Context, id string) error {
	return nil
}

func setupTestAlertService() (*AlertService, *mockDashboardRepo, *mockWidgetRepo, *mockAlertRepo) {
	dashRepo := &mockDashboardRepo{dashboards: make(map[string]*entity.Dashboard)}
	widgetRepo := &mockWidgetRepo{widgets: make(map[string]*entity.Widget)}
	alertRepo := &mockAlertRepo{alerts: make(map[string]*entity.DashboardAlert)}

	svc := NewAlertService(alertRepo, dashRepo, widgetRepo, nil, nil)
	return svc, dashRepo, widgetRepo, alertRepo
}

func TestAlertService_CreateValidDigest(t *testing.T) {
	svc, dashRepo, _, _ := setupTestAlertService()
	ctx := context.Background()

	dashRepo.dashboards["dash-1"] = &entity.Dashboard{
		ID:          "dash-1",
		WorkspaceID: "ws-1",
		ProjectID:   "proj-1",
		Title:       "Executive KPI",
	}

	req := request.CreateAlertRequest{
		Title:          "Daily 8am Digest",
		AlertType:      entity.AlertTypeScheduledDigest,
		CronExpression: "0 8 * * *",
		Channel:        entity.ChannelTelegram,
		EnvTag:         "production",
	}

	res, err := svc.Create(ctx, "ws-1", "dash-1", "user-1", req)
	if err != nil {
		t.Fatalf("unexpected error creating digest alert: %v", err)
	}
	if res.Title != "Daily 8am Digest" {
		t.Errorf("expected title %q, got %q", "Daily 8am Digest", res.Title)
	}
	if res.AlertType != entity.AlertTypeScheduledDigest {
		t.Errorf("expected alert_type %q, got %q", entity.AlertTypeScheduledDigest, res.AlertType)
	}
}

func TestAlertService_CreateValidThreshold(t *testing.T) {
	svc, dashRepo, widgetRepo, _ := setupTestAlertService()
	ctx := context.Background()

	dashRepo.dashboards["dash-1"] = &entity.Dashboard{
		ID:          "dash-1",
		WorkspaceID: "ws-1",
		ProjectID:   "proj-1",
	}
	widgetRepo.widgets["w-1"] = &entity.Widget{
		ID:          "w-1",
		DashboardID: "dash-1",
		RawSQL:      "SELECT count(*) FROM orders WHERE status = 'FAILED'",
	}

	wID := "w-1"
	op := ">="
	threshold := 50.0

	req := request.CreateAlertRequest{
		Title:             "High Error Rate Alert",
		AlertType:         entity.AlertTypeThresholdAlert,
		CronExpression:    "*/5 * * * *",
		WidgetID:          &wID,
		ConditionOperator: &op,
		ThresholdValue:    &threshold,
		Channel:           entity.ChannelSlack,
		EnvTag:            "production",
	}

	res, err := svc.Create(ctx, "ws-1", "dash-1", "user-1", req)
	if err != nil {
		t.Fatalf("unexpected error creating threshold alert: %v", err)
	}
	if res.Channel != entity.ChannelSlack {
		t.Errorf("expected channel SLACK, got %s", res.Channel)
	}
	if *res.ConditionOperator != ">=" || *res.ThresholdValue != 50.0 {
		t.Errorf("unexpected threshold condition: %v %v", res.ConditionOperator, res.ThresholdValue)
	}
}

func TestAlertService_RejectsInvalidCron(t *testing.T) {
	svc, dashRepo, _, _ := setupTestAlertService()
	ctx := context.Background()

	dashRepo.dashboards["dash-1"] = &entity.Dashboard{
		ID:          "dash-1",
		WorkspaceID: "ws-1",
	}

	req := request.CreateAlertRequest{
		Title:          "Bad Cron Alert",
		AlertType:      entity.AlertTypeScheduledDigest,
		CronExpression: "invalid-cron-format",
		Channel:        entity.ChannelTelegram,
	}

	_, err := svc.Create(ctx, "ws-1", "dash-1", "user-1", req)
	if err == nil {
		t.Fatal("expected error for invalid cron expression, got nil")
	}
}

func TestAlertService_RejectsInvalidChannel(t *testing.T) {
	svc, dashRepo, _, _ := setupTestAlertService()
	ctx := context.Background()

	dashRepo.dashboards["dash-1"] = &entity.Dashboard{
		ID:          "dash-1",
		WorkspaceID: "ws-1",
	}

	req := request.CreateAlertRequest{
		Title:          "Bad Channel",
		AlertType:      entity.AlertTypeScheduledDigest,
		CronExpression: "0 9 * * *",
		Channel:        "WHATSAPP", // unsupported
	}

	_, err := svc.Create(ctx, "ws-1", "dash-1", "user-1", req)
	if err == nil {
		t.Fatal("expected error for unsupported channel, got nil")
	}
}

func TestAlertService_ThresholdRequiresWidgetAndOperator(t *testing.T) {
	svc, dashRepo, _, _ := setupTestAlertService()
	ctx := context.Background()

	dashRepo.dashboards["dash-1"] = &entity.Dashboard{
		ID:          "dash-1",
		WorkspaceID: "ws-1",
	}

	// Missing widget_id
	req1 := request.CreateAlertRequest{
		Title:          "Missing Widget",
		AlertType:      entity.AlertTypeThresholdAlert,
		CronExpression: "*/10 * * * *",
		Channel:        entity.ChannelSlack,
	}
	_, err := svc.Create(ctx, "ws-1", "dash-1", "user-1", req1)
	if err == nil {
		t.Fatal("expected error when widget_id is missing for threshold alert")
	}

	// Invalid operator
	wID := "w-1"
	badOp := "LIKE"
	val := 10.0
	req2 := request.CreateAlertRequest{
		Title:             "Bad Op",
		AlertType:         entity.AlertTypeThresholdAlert,
		CronExpression:    "*/10 * * * *",
		WidgetID:          &wID,
		ConditionOperator: &badOp,
		ThresholdValue:    &val,
		Channel:           entity.ChannelSlack,
	}
	_, err = svc.Create(ctx, "ws-1", "dash-1", "user-1", req2)
	if err == nil {
		t.Fatal("expected error for bad operator")
	}
}

func TestAlertService_NonExistentDashboard(t *testing.T) {
	svc, _, _, _ := setupTestAlertService()
	ctx := context.Background()

	req := request.CreateAlertRequest{
		Title:          "Orphan Alert",
		AlertType:      entity.AlertTypeScheduledDigest,
		CronExpression: "0 0 * * *",
		Channel:        entity.ChannelEmail,
	}
	_, err := svc.Create(ctx, "ws-1", "non-existent-dash", "user-1", req)
	if err == nil {
		t.Fatal("expected not found error for non-existent dashboard")
	}
	appErr, ok := err.(*customerror.AppError)
	if !ok || appErr.Code != customerror.ErrNotFound {
		t.Errorf("expected 404 ErrNotFound, got %v", err)
	}
}
