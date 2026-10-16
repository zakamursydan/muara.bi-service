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

type QueryHandler struct {
	service service.QueryServiceProvider
	guard   *middleware.Guard
}

func NewQueryHandler(service service.QueryServiceProvider, guard *middleware.Guard) *QueryHandler {
	return &QueryHandler{service: service, guard: guard}
}

// RegisterRoutes mounts query routes relative to the
// /api/v1/workspaces/{wsId} router.
func (h *QueryHandler) RegisterRoutes(r chi.Router) {
	// Project membership and row-level filters are resolved from the request
	// body inside the service, so the guard here only checks the workspace role.
	r.With(h.guard.Require(entity.PermQueryExecute)).Post("/query/execute", h.Execute)
}

func (h *QueryHandler) Execute(w http.ResponseWriter, r *http.Request) {
	wsID := chi.URLParam(r, "wsId")

	var req request.ExecuteQueryRequest
	if !DecodeJSON(w, r, &req) {
		return
	}

	if r.Header.Get("X-Force-Refresh") == "true" {
		req.ForceRefresh = true
	}

	res, err := h.service.ExecuteQuery(
		r.Context(),
		wsID,
		middleware.GetUserIDFromContext(r.Context()),
		req,
		middleware.GetUserAttributesMapFromContext(r.Context()),
	)
	if err != nil {
		WriteServiceError(w, r, err)
		return
	}

	httputil.WriteSuccessResponse(w, r, "Query executed successfully", res)
}
