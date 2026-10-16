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

type AIHandler struct {
	service service.AIServiceProvider
	guard   *middleware.Guard
}

func NewAIHandler(service service.AIServiceProvider, guard *middleware.Guard) *AIHandler {
	return &AIHandler{service: service, guard: guard}
}

func (h *AIHandler) RegisterRoutes(r chi.Router) {
	r.With(h.guard.Require(entity.PermDashboardRead)).Get("/ai/settings", h.GetSettings)
	r.With(h.guard.Require(entity.PermConnectionWrite)).Put("/ai/settings", h.UpdateSettings)
	r.With(h.guard.Require(entity.PermQueryExecute)).Post("/ai/generate-sql", h.GenerateSQL)
	r.With(h.guard.Require(entity.PermDashboardWrite)).Post("/ai/generate-dashboard", h.GenerateDashboard)
}

func (h *AIHandler) GetSettings(w http.ResponseWriter, r *http.Request) {
	wsID := chi.URLParam(r, "wsId")
	res, err := h.service.GetSettings(r.Context(), wsID)
	if err != nil {
		WriteServiceError(w, r, err)
		return
	}
	httputil.WriteSuccessResponse(w, r, "AI settings retrieved successfully", res)
}

func (h *AIHandler) UpdateSettings(w http.ResponseWriter, r *http.Request) {
	wsID := chi.URLParam(r, "wsId")
	var req request.UpdateAISettingsRequest
	if !DecodeJSON(w, r, &req) {
		return
	}

	res, err := h.service.UpdateSettings(r.Context(), wsID, req)
	if err != nil {
		WriteServiceError(w, r, err)
		return
	}
	httputil.WriteSuccessResponse(w, r, "AI settings updated successfully", res)
}

func (h *AIHandler) GenerateSQL(w http.ResponseWriter, r *http.Request) {
	wsID := chi.URLParam(r, "wsId")
	var req request.GenerateSQLRequest
	if !DecodeJSON(w, r, &req) {
		return
	}

	res, err := h.service.GenerateSQL(r.Context(), wsID, req)
	if err != nil {
		WriteServiceError(w, r, err)
		return
	}
	httputil.WriteSuccessResponse(w, r, "SQL generated successfully", res)
}

func (h *AIHandler) GenerateDashboard(w http.ResponseWriter, r *http.Request) {
	wsID := chi.URLParam(r, "wsId")
	var req request.GenerateDashboardRequest
	if !DecodeJSON(w, r, &req) {
		return
	}

	res, err := h.service.GenerateDashboard(r.Context(), wsID, req)
	if err != nil {
		WriteServiceError(w, r, err)
		return
	}
	httputil.WriteSuccessResponse(w, r, "Dashboard generated successfully", res)
}
