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

// AlertHandler manages scheduled digests and threshold anomaly monitors.
// Every path is nested under {dashId} so RBAC guard derives project scope.
type AlertHandler struct {
	service service.AlertServiceProvider
	guard   *middleware.Guard
}

func NewAlertHandler(service service.AlertServiceProvider, guard *middleware.Guard) *AlertHandler {
	return &AlertHandler{service: service, guard: guard}
}

func (h *AlertHandler) RegisterRoutes(r chi.Router) {
	r.With(h.guard.Require(entity.PermDashboardWrite)).Post("/dashboards/{dashId}/alerts", h.Create)
	r.With(h.guard.Require(entity.PermDashboardRead)).Get("/dashboards/{dashId}/alerts", h.List)
	r.With(h.guard.Require(entity.PermDashboardRead)).Get("/dashboards/{dashId}/alerts/{alertId}", h.Get)
	r.With(h.guard.Require(entity.PermDashboardWrite)).Put("/dashboards/{dashId}/alerts/{alertId}", h.Update)
	r.With(h.guard.Require(entity.PermDashboardWrite)).Delete("/dashboards/{dashId}/alerts/{alertId}", h.Delete)
	r.With(h.guard.Require(entity.PermDashboardWrite)).Post("/dashboards/{dashId}/alerts/{alertId}/test", h.Test)
}

func (h *AlertHandler) Create(w http.ResponseWriter, r *http.Request) {
	wsID := chi.URLParam(r, "wsId")
	dashID := chi.URLParam(r, "dashId")
	userID := middleware.GetUserIDFromContext(r.Context())

	var req request.CreateAlertRequest
	if !DecodeJSON(w, r, &req) {
		return
	}

	res, err := h.service.Create(r.Context(), wsID, dashID, userID, req)
	if err != nil {
		WriteServiceError(w, r, err)
		return
	}

	httputil.WriteCreatedResponse(w, r, "Alert created successfully", res)
}

func (h *AlertHandler) List(w http.ResponseWriter, r *http.Request) {
	wsID := chi.URLParam(r, "wsId")
	dashID := chi.URLParam(r, "dashId")

	res, err := h.service.ListByDashboard(r.Context(), wsID, dashID)
	if err != nil {
		WriteServiceError(w, r, err)
		return
	}

	httputil.WriteSuccessResponse(w, r, "Alerts retrieved successfully", res)
}

func (h *AlertHandler) Get(w http.ResponseWriter, r *http.Request) {
	wsID := chi.URLParam(r, "wsId")
	alertID := chi.URLParam(r, "alertId")

	res, err := h.service.GetByID(r.Context(), wsID, alertID)
	if err != nil {
		WriteServiceError(w, r, err)
		return
	}

	httputil.WriteSuccessResponse(w, r, "Alert retrieved successfully", res)
}

func (h *AlertHandler) Update(w http.ResponseWriter, r *http.Request) {
	wsID := chi.URLParam(r, "wsId")
	alertID := chi.URLParam(r, "alertId")

	var req request.UpdateAlertRequest
	if !DecodeJSON(w, r, &req) {
		return
	}

	res, err := h.service.Update(r.Context(), wsID, alertID, req)
	if err != nil {
		WriteServiceError(w, r, err)
		return
	}

	httputil.WriteSuccessResponse(w, r, "Alert updated successfully", res)
}

func (h *AlertHandler) Delete(w http.ResponseWriter, r *http.Request) {
	wsID := chi.URLParam(r, "wsId")
	alertID := chi.URLParam(r, "alertId")

	if err := h.service.Delete(r.Context(), wsID, alertID); err != nil {
		WriteServiceError(w, r, err)
		return
	}

	httputil.WriteSuccessResponse(w, r, "Alert deleted successfully", nil)
}

func (h *AlertHandler) Test(w http.ResponseWriter, r *http.Request) {
	wsID := chi.URLParam(r, "wsId")
	alertID := chi.URLParam(r, "alertId")

	if err := h.service.TestAlert(r.Context(), wsID, alertID); err != nil {
		WriteServiceError(w, r, err)
		return
	}

	httputil.WriteSuccessResponse(w, r, "Test alert queued for delivery", nil)
}
