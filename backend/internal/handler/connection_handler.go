package handler

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"muara.bi/core/internal/domain/entity"
	"muara.bi/core/internal/dto/request"
	"muara.bi/core/internal/middleware"
	"muara.bi/core/internal/service"
	"muara.bi/core/pkg/httputil"
)

type ConnectionHandler struct {
	service service.ConnectionServiceProvider
	guard   *middleware.Guard
}

func NewConnectionHandler(service service.ConnectionServiceProvider, guard *middleware.Guard) *ConnectionHandler {
	return &ConnectionHandler{service: service, guard: guard}
}

// RegisterRoutes mounts connection routes relative to the
// /api/v1/workspaces/{wsId} router.
func (h *ConnectionHandler) RegisterRoutes(r chi.Router) {
	r.With(h.guard.Require(entity.PermConnectionWrite)).Post("/connections", h.Create)
	r.With(h.guard.Require(entity.PermConnectionRead)).Get("/connections", h.List)
	r.With(h.guard.Require(entity.PermConnectionWrite)).Post("/connections/{connId}/environments", h.UpsertEnvironment)
	r.With(h.guard.Require(entity.PermConnectionRead)).Post("/connections/{connId}/environments/{envTag}/test", h.Test)
	r.With(h.guard.Require(entity.PermConnectionWrite)).Post("/connections/{connId}/environments/{envTag}/sync-schema", h.SyncSchema)
}

func (h *ConnectionHandler) Create(w http.ResponseWriter, r *http.Request) {
	wsID := chi.URLParam(r, "wsId")
	var req request.CreateConnectionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.WriteBadRequestResponse(w, r, "Invalid request payload", err.Error())
		return
	}

	res, err := h.service.CreateConnection(r.Context(), wsID, req)
	if err != nil {
		httputil.WriteInternalServerErrorResponse(w, r, err.Error(), nil)
		return
	}

	httputil.WriteCreatedResponse(w, r, "Data connection created successfully", res)
}

func (h *ConnectionHandler) List(w http.ResponseWriter, r *http.Request) {
	wsID := chi.URLParam(r, "wsId")
	res, err := h.service.ListConnections(r.Context(), wsID)
	if err != nil {
		httputil.WriteInternalServerErrorResponse(w, r, err.Error(), nil)
		return
	}

	httputil.WriteSuccessResponse(w, r, "Data connections fetched successfully", res)
}

func (h *ConnectionHandler) UpsertEnvironment(w http.ResponseWriter, r *http.Request) {
	wsID := chi.URLParam(r, "wsId")
	connID := chi.URLParam(r, "connId")

	var req request.UpsertEnvironmentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.WriteBadRequestResponse(w, r, "Invalid request payload", err.Error())
		return
	}

	res, err := h.service.UpsertEnvironment(r.Context(), wsID, connID, req)
	if err != nil {
		httputil.WriteInternalServerErrorResponse(w, r, err.Error(), nil)
		return
	}

	httputil.WriteSuccessResponse(w, r, "Environment profile saved successfully", res)
}

func (h *ConnectionHandler) Test(w http.ResponseWriter, r *http.Request) {
	wsID := chi.URLParam(r, "wsId")
	connID := chi.URLParam(r, "connId")
	envTag := chi.URLParam(r, "envTag")

	res, err := h.service.TestConnection(r.Context(), wsID, connID, envTag)
	if err != nil {
		httputil.WriteInternalServerErrorResponse(w, r, err.Error(), nil)
		return
	}

	httputil.WriteSuccessResponse(w, r, "Connection test completed", res)
}

func (h *ConnectionHandler) SyncSchema(w http.ResponseWriter, r *http.Request) {
	wsID := chi.URLParam(r, "wsId")
	connID := chi.URLParam(r, "connId")
	envTag := chi.URLParam(r, "envTag")

	if err := h.service.SyncSchema(r.Context(), wsID, connID, envTag); err != nil {
		httputil.WriteInternalServerErrorResponse(w, r, err.Error(), nil)
		return
	}

	httputil.WriteSuccessResponse(w, r, "Schema synced successfully", nil)
}
