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

// WidgetHandler serves canvas card routes. Paths are relative to the
// /api/v1/workspaces/{wsId} router.
type WidgetHandler struct {
	service service.WidgetServiceProvider
	guard   *middleware.Guard
}

func NewWidgetHandler(service service.WidgetServiceProvider, guard *middleware.Guard) *WidgetHandler {
	return &WidgetHandler{service: service, guard: guard}
}

func (h *WidgetHandler) RegisterRoutes(r chi.Router) {
	r.With(h.guard.Require(entity.PermDashboardWrite)).Post("/dashboards/{dashId}/widgets", h.Create)
	r.With(h.guard.Require(entity.PermDashboardRead)).Get("/widgets/{widgetId}", h.Get)
	r.With(h.guard.Require(entity.PermDashboardWrite)).Put("/widgets/{widgetId}", h.Update)
	r.With(h.guard.Require(entity.PermDashboardWrite)).Delete("/widgets/{widgetId}", h.Delete)
}

func (h *WidgetHandler) Create(w http.ResponseWriter, r *http.Request) {
	wsID := chi.URLParam(r, "wsId")
	dashID := chi.URLParam(r, "dashId")

	var req request.CreateWidgetRequest
	if !DecodeJSON(w, r, &req) {
		return
	}

	res, err := h.service.Create(r.Context(), wsID, dashID, req)
	if err != nil {
		WriteServiceError(w, r, err)
		return
	}

	httputil.WriteCreatedResponse(w, r, "Widget created successfully", res)
}

func (h *WidgetHandler) Get(w http.ResponseWriter, r *http.Request) {
	wsID := chi.URLParam(r, "wsId")
	widgetID := chi.URLParam(r, "widgetId")

	res, err := h.service.Get(r.Context(), wsID, widgetID)
	if err != nil {
		WriteServiceError(w, r, err)
		return
	}

	httputil.WriteSuccessResponse(w, r, "Widget fetched successfully", res)
}

func (h *WidgetHandler) Update(w http.ResponseWriter, r *http.Request) {
	wsID := chi.URLParam(r, "wsId")
	widgetID := chi.URLParam(r, "widgetId")

	var req request.UpdateWidgetRequest
	if !DecodeJSON(w, r, &req) {
		return
	}

	res, err := h.service.Update(r.Context(), wsID, widgetID, req)
	if err != nil {
		WriteServiceError(w, r, err)
		return
	}

	httputil.WriteSuccessResponse(w, r, "Widget updated successfully", res)
}

func (h *WidgetHandler) Delete(w http.ResponseWriter, r *http.Request) {
	wsID := chi.URLParam(r, "wsId")
	widgetID := chi.URLParam(r, "widgetId")

	if err := h.service.Delete(r.Context(), wsID, widgetID); err != nil {
		WriteServiceError(w, r, err)
		return
	}

	httputil.WriteSuccessResponse(w, r, "Widget deleted successfully", nil)
}
