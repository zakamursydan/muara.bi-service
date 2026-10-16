package middleware

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"muara.bi/core/internal/domain/entity"
	"muara.bi/core/internal/duckdb"
	"muara.bi/core/internal/service"
)

type stubAccess struct {
	access     *service.Access
	resolveErr error
	dashProj   string
	widgetProj string
	scopes     []service.Scope
}

func (s *stubAccess) Resolve(_ context.Context, scope service.Scope) (*service.Access, error) {
	s.scopes = append(s.scopes, scope)
	if s.resolveErr != nil {
		return nil, s.resolveErr
	}
	return s.access, nil
}

func (s *stubAccess) ProjectForDashboard(_ context.Context, _, _ string) (string, error) {
	return s.dashProj, nil
}

func (s *stubAccess) ProjectForWidget(_ context.Context, _, _ string) (string, error) {
	return s.widgetProj, nil
}

// newGuardTestRouter mirrors the production shape: a JWT-protected workspace
// subrouter with guarded inline routes.
func newGuardTestRouter(guard *Guard, onRequest func(*http.Request)) chi.Router {
	authenticate := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := context.WithValue(r.Context(), UserIDKey, "user-1")
			ctx = context.WithValue(ctx, UserAttributesKey, json.RawMessage(`{"branch_id":"SUB_01"}`))
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}

	ok := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if onRequest != nil {
			onRequest(r)
		}
		w.WriteHeader(http.StatusOK)
	})

	router := chi.NewRouter()
	router.Route("/api/v1/workspaces/{wsId}", func(ws chi.Router) {
		ws.Use(authenticate)
		ws.With(guard.Require(entity.PermDashboardRead)).Get("/projects/{projId}/dashboards", ok)
		ws.With(guard.Require(entity.PermDashboardWrite)).Put("/dashboards/{dashId}", ok)
		ws.With(guard.Require(entity.PermDashboardWrite)).Delete("/widgets/{widgetId}", ok)
	})
	return router
}

func TestGuardAllowsPermittedRequest(t *testing.T) {
	store := &stubAccess{access: &service.Access{
		WorkspaceRole: entity.RoleEditor,
		Permissions:   entity.NewPermissionSet(entity.PermDashboardRead),
	}}
	router := newGuardTestRouter(NewGuard(store), nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/workspaces/ws-1/projects/proj-1/dashboards", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body: %s)", rec.Code, rec.Body.String())
	}
	if len(store.scopes) != 1 {
		t.Fatalf("expected one resolve call, got %d", len(store.scopes))
	}
	scope := store.scopes[0]
	if scope.WorkspaceID != "ws-1" || scope.ProjectID != "proj-1" || scope.UserID != "user-1" {
		t.Fatalf("unexpected scope: %+v", scope)
	}
	if scope.Attributes["branch_id"] != "SUB_01" {
		t.Fatalf("expected JWT attributes on scope, got %+v", scope.Attributes)
	}
}

func TestGuardRejectsMissingPermission(t *testing.T) {
	store := &stubAccess{access: &service.Access{
		WorkspaceRole: entity.RoleViewer,
		Permissions:   entity.NewPermissionSet(entity.PermDashboardRead),
	}}
	router := newGuardTestRouter(NewGuard(store), nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/workspaces/ws-1/projects/proj-1/dashboards", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("viewer holds dashboard:read, status = %d", rec.Code)
	}

	// Same router, but the write permission is not granted.
	store2 := &stubAccess{access: &service.Access{Permissions: entity.NewPermissionSet(entity.PermDashboardRead)}}
	router2 := newGuardTestRouter(NewGuard(store2), nil)
	req2 := httptest.NewRequest(http.MethodPut, "/api/v1/workspaces/ws-1/dashboards/dash-1", nil)
	rec2 := httptest.NewRecorder()
	router2.ServeHTTP(rec2, req2)

	if rec2.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403 (body: %s)", rec2.Code, rec2.Body.String())
	}
}

func TestGuardDerivesProjectFromDashboardParam(t *testing.T) {
	store := &stubAccess{
		access:   &service.Access{Permissions: entity.NewPermissionSet(entity.PermDashboardWrite)},
		dashProj: "proj-9",
	}
	router := newGuardTestRouter(NewGuard(store), nil)

	req := httptest.NewRequest(http.MethodPut, "/api/v1/workspaces/ws-1/dashboards/dash-1", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if store.scopes[0].ProjectID != "proj-9" {
		t.Fatalf("expected project resolved from dashboard, got %q", store.scopes[0].ProjectID)
	}
}

func TestGuardDerivesProjectFromWidgetParam(t *testing.T) {
	store := &stubAccess{
		access:     &service.Access{Permissions: entity.NewPermissionSet(entity.PermDashboardWrite)},
		widgetProj: "proj-7",
	}
	router := newGuardTestRouter(NewGuard(store), nil)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/workspaces/ws-1/widgets/w-1", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if store.scopes[0].ProjectID != "proj-7" {
		t.Fatalf("expected project resolved from widget, got %q", store.scopes[0].ProjectID)
	}
}

func TestGuardPublishesAccessOnContext(t *testing.T) {
	granted := &service.Access{
		WorkspaceRole: entity.RoleViewer,
		ProjectRole:   entity.ProjectRoleViewer,
		Permissions:   entity.NewPermissionSet(entity.PermDashboardRead),
		RLSRules:      []duckdb.RLSRule{{Column: "branch_id", Operator: "=", Value: "SUB_01"}},
	}
	store := &stubAccess{access: granted}

	var seen *service.Access
	router := newGuardTestRouter(NewGuard(store), func(r *http.Request) {
		seen = GetAccessFromContext(r.Context())
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/workspaces/ws-1/projects/proj-1/dashboards", nil)
	router.ServeHTTP(httptest.NewRecorder(), req)

	if seen == nil {
		t.Fatal("expected resolved access on the request context")
	}
	if len(seen.RLSRules) != 1 || seen.RLSRules[0].Value != "SUB_01" {
		t.Fatalf("unexpected RLS rules on context: %+v", seen.RLSRules)
	}
}

func TestGetAccessFromContextIsNilWhenUnguarded(t *testing.T) {
	if access := GetAccessFromContext(context.Background()); access != nil {
		t.Fatalf("expected nil access, got %+v", access)
	}
}
