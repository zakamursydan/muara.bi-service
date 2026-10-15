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

type SemanticHandler struct {
	service service.SemanticServiceProvider
}

func NewSemanticHandler(service service.SemanticServiceProvider) *SemanticHandler {
	return &SemanticHandler{service: service}
}

func (h *SemanticHandler) RegisterRoutes(r chi.Router) {
	r.Route("/api/v1/workspaces/{wsId}", func(r chi.Router) {
		r.Post("/projects/{projId}/semantic-models", h.Create)
		r.Get("/projects/{projId}/semantic-models", h.List)
		r.Get("/semantic-models/{id}", h.Get)
		r.Put("/semantic-models/{id}", h.Update)
		r.Delete("/semantic-models/{id}", h.Delete)
	})
}

func (h *SemanticHandler) Create(w http.ResponseWriter, r *http.Request) {
	wsID := chi.URLParam(r, "wsId")
	projID := chi.URLParam(r, "projId")
	userID := middleware.GetUserIDFromContext(r.Context())

	var req request.CreateSemanticModelRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.WriteBadRequestResponse(w, r, "Invalid request payload", err.Error())
		return
	}

	res, err := h.service.Create(r.Context(), wsID, projID, userID, req)
	if err != nil {
		httputil.WriteInternalServerErrorResponse(w, r, err.Error(), nil)
		return
	}

	httputil.WriteCreatedResponse(w, r, "Semantic model created successfully", res)
}

func (h *SemanticHandler) List(w http.ResponseWriter, r *http.Request) {
	wsID := chi.URLParam(r, "wsId")
	projID := chi.URLParam(r, "projId")

	res, err := h.service.List(r.Context(), wsID, projID)
	if err != nil {
		httputil.WriteInternalServerErrorResponse(w, r, err.Error(), nil)
		return
	}

	httputil.WriteSuccessResponse(w, r, "Semantic models fetched successfully", res)
}

func (h *SemanticHandler) Get(w http.ResponseWriter, r *http.Request) {
	wsID := chi.URLParam(r, "wsId")
	id := chi.URLParam(r, "id")

	res, err := h.service.Get(r.Context(), wsID, id)
	if err != nil {
		httputil.WriteInternalServerErrorResponse(w, r, err.Error(), nil)
		return
	}

	httputil.WriteSuccessResponse(w, r, "Semantic model fetched successfully", res)
}

func (h *SemanticHandler) Update(w http.ResponseWriter, r *http.Request) {
	wsID := chi.URLParam(r, "wsId")
	id := chi.URLParam(r, "id")

	var req request.UpdateSemanticModelRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.WriteBadRequestResponse(w, r, "Invalid request payload", err.Error())
		return
	}

	if err := h.service.Update(r.Context(), wsID, id, req); err != nil {
		httputil.WriteInternalServerErrorResponse(w, r, err.Error(), nil)
		return
	}

	httputil.WriteSuccessResponse(w, r, "Semantic model updated successfully", nil)
}

func (h *SemanticHandler) Delete(w http.ResponseWriter, r *http.Request) {
	wsID := chi.URLParam(r, "wsId")
	id := chi.URLParam(r, "id")

	if err := h.service.Delete(r.Context(), wsID, id); err != nil {
		httputil.WriteInternalServerErrorResponse(w, r, err.Error(), nil)
		return
	}

	httputil.WriteSuccessResponse(w, r, "Semantic model deleted successfully", nil)
}
