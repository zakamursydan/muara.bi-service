package handler

import (
	"net/http"
	"sort"

	"github.com/go-chi/chi/v5"
	"muara.bi/core/internal/dto/response"
	"muara.bi/core/internal/middleware"
	"muara.bi/core/internal/service"
	"muara.bi/core/pkg/httputil"
)

// AccessHandler reports the caller's own resolved role and permissions.
// Paths are relative to the /api/v1/workspaces/{wsId} router.
type AccessHandler struct {
	access service.AccessServiceProvider
}

func NewAccessHandler(access service.AccessServiceProvider) *AccessHandler {
	return &AccessHandler{access: access}
}

// RegisterRoutes has no Guard: a non-member holds zero permissions but still
// needs to learn that, otherwise the UI cannot tell "no access" from "failed
// to load".
func (h *AccessHandler) RegisterRoutes(r chi.Router) {
	r.Get("/access", h.Get)
}

func (h *AccessHandler) Get(w http.ResponseWriter, r *http.Request) {
	wsID := chi.URLParam(r, "wsId")

	projectID := r.URL.Query().Get("project_id")

	access, err := h.access.Resolve(r.Context(), service.Scope{
		WorkspaceID: wsID,
		UserID:      middleware.GetUserIDFromContext(r.Context()),
		Attributes:  middleware.GetUserAttributesMapFromContext(r.Context()),
		ProjectID:   projectID,
	})
	if err != nil {
		WriteServiceError(w, r, err)
		return
	}

	permissions := make([]string, 0, len(access.Permissions))
	for permission := range access.Permissions {
		permissions = append(permissions, string(permission))
	}
	sort.Strings(permissions)

	// Column names only: the bound row values stay inside the query engine.
	rlsColumns := make([]string, 0, len(access.RLSRules))
	for _, rule := range access.RLSRules {
		rlsColumns = append(rlsColumns, rule.Column)
	}
	sort.Strings(rlsColumns)

	httputil.WriteSuccessResponse(w, r, "Access resolved successfully", response.AccessResponse{
		WorkspaceID:   wsID,
		ProjectID:     projectID,
		WorkspaceRole: access.WorkspaceRole,
		ProjectRole:   access.ProjectRole,
		Permissions:   permissions,
		RLSColumns:    rlsColumns,
	})
}
