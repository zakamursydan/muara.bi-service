package handler

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"muara.bi/core/internal/dto/request"
	"muara.bi/core/internal/middleware"
	"muara.bi/core/internal/service"
	"muara.bi/core/pkg/httputil"
)

type QueryHandler struct {
	service service.QueryServiceProvider
}

func NewQueryHandler(service service.QueryServiceProvider) *QueryHandler {
	return &QueryHandler{service: service}
}

func (h *QueryHandler) RegisterRoutes(r chi.Router) {
	r.Route("/api/v1/workspaces/{wsId}/query", func(r chi.Router) {
		r.Post("/execute", h.Execute)
	})
}

func (h *QueryHandler) Execute(w http.ResponseWriter, r *http.Request) {
	wsID := chi.URLParam(r, "wsId")

	var req request.ExecuteQueryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.WriteBadRequestResponse(w, r, "Invalid request payload", err.Error())
		return
	}

	// Check X-Force-Refresh header
	if r.Header.Get("X-Force-Refresh") == "true" {
		req.ForceRefresh = true
	}

	// Dynamic RLS user attributes from JWT Context
	var userAttrs map[string]any
	rawAttrs := middleware.GetUserAttributesFromContext(r.Context())
	_ = json.Unmarshal(rawAttrs, &userAttrs)

	res, err := h.service.ExecuteQuery(r.Context(), wsID, req, userAttrs)
	if err != nil {
		httputil.WriteInternalServerErrorResponse(w, r, err.Error(), nil)
		return
	}

	httputil.WriteSuccessResponse(w, r, "Query executed successfully", res)
}
