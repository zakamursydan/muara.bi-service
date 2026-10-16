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

// ProjectHandler serves project catalog and granular project membership routes.
// Paths are relative to the /api/v1/workspaces/{wsId} router.
type ProjectHandler struct {
	service service.ProjectServiceProvider
	guard   *middleware.Guard
}

func NewProjectHandler(service service.ProjectServiceProvider, guard *middleware.Guard) *ProjectHandler {
	return &ProjectHandler{service: service, guard: guard}
}

func (h *ProjectHandler) RegisterRoutes(r chi.Router) {
	r.With(h.guard.Require(entity.PermProjectRead)).Get("/projects", h.List)
	r.With(h.guard.Require(entity.PermProjectWrite)).Post("/projects", h.Create)
	r.With(h.guard.Require(entity.PermProjectRead)).Get("/projects/{projId}", h.Get)
	r.With(h.guard.Require(entity.PermProjectRead)).Get("/projects/{projId}/members", h.ListMembers)
	r.With(h.guard.Require(entity.PermMemberManage)).Post("/projects/{projId}/members", h.AssignMember)
}

func (h *ProjectHandler) List(w http.ResponseWriter, r *http.Request) {
	wsID := chi.URLParam(r, "wsId")

	res, err := h.service.List(r.Context(), wsID)
	if err != nil {
		WriteServiceError(w, r, err)
		return
	}

	httputil.WriteSuccessResponse(w, r, "Projects fetched successfully", res)
}

func (h *ProjectHandler) Create(w http.ResponseWriter, r *http.Request) {
	wsID := chi.URLParam(r, "wsId")
	userID := middleware.GetUserIDFromContext(r.Context())

	var req request.CreateProjectRequest
	if !DecodeJSON(w, r, &req) {
		return
	}

	res, err := h.service.Create(r.Context(), wsID, userID, req)
	if err != nil {
		WriteServiceError(w, r, err)
		return
	}

	httputil.WriteCreatedResponse(w, r, "Project created successfully", res)
}

func (h *ProjectHandler) Get(w http.ResponseWriter, r *http.Request) {
	wsID := chi.URLParam(r, "wsId")
	projID := chi.URLParam(r, "projId")

	res, err := h.service.Get(r.Context(), wsID, projID)
	if err != nil {
		WriteServiceError(w, r, err)
		return
	}

	httputil.WriteSuccessResponse(w, r, "Project fetched successfully", res)
}

func (h *ProjectHandler) ListMembers(w http.ResponseWriter, r *http.Request) {
	wsID := chi.URLParam(r, "wsId")
	projID := chi.URLParam(r, "projId")

	res, err := h.service.ListMembers(r.Context(), wsID, projID)
	if err != nil {
		WriteServiceError(w, r, err)
		return
	}

	httputil.WriteSuccessResponse(w, r, "Project members fetched successfully", res)
}

func (h *ProjectHandler) AssignMember(w http.ResponseWriter, r *http.Request) {
	wsID := chi.URLParam(r, "wsId")
	projID := chi.URLParam(r, "projId")

	var req request.AssignProjectMemberRequest
	if !DecodeJSON(w, r, &req) {
		return
	}

	res, err := h.service.AssignMember(r.Context(), wsID, projID, req)
	if err != nil {
		WriteServiceError(w, r, err)
		return
	}

	httputil.WriteCreatedResponse(w, r, "Project member assigned successfully", res)
}
