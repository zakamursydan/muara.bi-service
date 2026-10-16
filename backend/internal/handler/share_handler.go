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

// ShareHandler manages share links for one dashboard. Every path is nested under
// {dashId} so the RBAC guard resolves project scope from the dashboard itself.
// Paths are relative to the /api/v1/workspaces/{wsId} router.
type ShareHandler struct {
	service service.ShareServiceProvider
	guard   *middleware.Guard
}

func NewShareHandler(service service.ShareServiceProvider, guard *middleware.Guard) *ShareHandler {
	return &ShareHandler{service: service, guard: guard}
}

func (h *ShareHandler) RegisterRoutes(r chi.Router) {
	// Publishing data outside the tenant is an admin act, so listing and revoking
	// are gated on share:publish too: the token in each row is a bearer credential.
	r.With(h.guard.Require(entity.PermSharePublish)).Post("/dashboards/{dashId}/shares", h.Create)
	r.With(h.guard.Require(entity.PermSharePublish)).Get("/dashboards/{dashId}/shares", h.List)
	r.With(h.guard.Require(entity.PermSharePublish)).Delete("/dashboards/{dashId}/shares/{shareId}", h.Revoke)
}

func (h *ShareHandler) Create(w http.ResponseWriter, r *http.Request) {
	wsID := chi.URLParam(r, "wsId")
	dashID := chi.URLParam(r, "dashId")
	userID := middleware.GetUserIDFromContext(r.Context())

	var req request.CreateShareRequest
	if !DecodeJSON(w, r, &req) {
		return
	}

	res, err := h.service.Create(r.Context(), wsID, dashID, userID, req)
	if err != nil {
		WriteServiceError(w, r, err)
		return
	}

	httputil.WriteCreatedResponse(w, r, "Share link created successfully", res)
}

func (h *ShareHandler) List(w http.ResponseWriter, r *http.Request) {
	wsID := chi.URLParam(r, "wsId")
	dashID := chi.URLParam(r, "dashId")

	res, err := h.service.List(r.Context(), wsID, dashID)
	if err != nil {
		WriteServiceError(w, r, err)
		return
	}

	httputil.WriteSuccessResponse(w, r, "Share links fetched successfully", res)
}

func (h *ShareHandler) Revoke(w http.ResponseWriter, r *http.Request) {
	wsID := chi.URLParam(r, "wsId")
	dashID := chi.URLParam(r, "dashId")
	shareID := chi.URLParam(r, "shareId")

	if err := h.service.Revoke(r.Context(), wsID, dashID, shareID); err != nil {
		WriteServiceError(w, r, err)
		return
	}

	httputil.WriteSuccessResponse(w, r, "Share link revoked successfully", nil)
}
