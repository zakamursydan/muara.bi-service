package handler

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"muara.bi/core/internal/domain/entity"
	"muara.bi/core/internal/dto/request"
	"muara.bi/core/internal/middleware"
	"muara.bi/core/internal/service"
	"muara.bi/core/pkg/httputil"
)

// DashboardHandler serves the project dashboard catalog and the canvas payload.
// Paths are relative to the /api/v1/workspaces/{wsId} router.
type DashboardHandler struct {
	service service.DashboardServiceProvider
	guard   *middleware.Guard
}

func NewDashboardHandler(service service.DashboardServiceProvider, guard *middleware.Guard) *DashboardHandler {
	return &DashboardHandler{service: service, guard: guard}
}

func (h *DashboardHandler) RegisterRoutes(r chi.Router) {
	r.With(h.guard.Require(entity.PermDashboardRead)).Get("/projects/{projId}/dashboards", h.List)
	r.With(h.guard.Require(entity.PermDashboardWrite)).Post("/projects/{projId}/dashboards", h.Create)
	r.With(h.guard.Require(entity.PermDashboardRead)).Get("/dashboards/{dashId}", h.Get)
	r.With(h.guard.Require(entity.PermDashboardWrite)).Put("/dashboards/{dashId}", h.Update)
}

func (h *DashboardHandler) List(w http.ResponseWriter, r *http.Request) {
	wsID := chi.URLParam(r, "wsId")
	projID := chi.URLParam(r, "projId")

	res, err := h.service.List(r.Context(), wsID, projID)
	if err != nil {
		WriteServiceError(w, r, err)
		return
	}

	httputil.WriteSuccessResponse(w, r, "Dashboards fetched successfully", res)
}

func (h *DashboardHandler) Create(w http.ResponseWriter, r *http.Request) {
	wsID := chi.URLParam(r, "wsId")
	projID := chi.URLParam(r, "projId")
	userID := middleware.GetUserIDFromContext(r.Context())

	var req request.CreateDashboardRequest
	if !DecodeJSON(w, r, &req) {
		return
	}

	res, err := h.service.Create(r.Context(), wsID, projID, userID, req)
	if err != nil {
		WriteServiceError(w, r, err)
		return
	}

	httputil.WriteCreatedResponse(w, r, "Dashboard created successfully", res)
}

func (h *DashboardHandler) Get(w http.ResponseWriter, r *http.Request) {
	wsID := chi.URLParam(r, "wsId")
	dashID := chi.URLParam(r, "dashId")

	res, err := h.service.Get(r.Context(), wsID, dashID)
	if err != nil {
		WriteServiceError(w, r, err)
		return
	}

	httputil.WriteSuccessResponse(w, r, "Dashboard fetched successfully", res)
}

// Update receives the debounced auto-save payload emitted by the canvas.
func (h *DashboardHandler) Update(w http.ResponseWriter, r *http.Request) {
	wsID := chi.URLParam(r, "wsId")
	dashID := chi.URLParam(r, "dashId")

	var req request.UpdateDashboardRequest
	if !DecodeJSON(w, r, &req) {
		return
	}

	res, err := h.service.Update(r.Context(), wsID, dashID, req)
	if err != nil {
		WriteServiceError(w, r, err)
		return
	}

	httputil.WriteSuccessResponse(w, r, "Dashboard updated successfully", res)
}
