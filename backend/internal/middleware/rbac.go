package middleware

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"muara.bi/core/internal/domain/entity"
	"muara.bi/core/internal/service"
	"muara.bi/core/pkg/httputil"
)

const accessKey contextKey = "access"

// Guard enforces granular RBAC. It derives the project scope from whichever
// identifier the route carries ({projId}, {dashId} or {widgetId}) so handlers
// stay free of authorization logic.
type Guard struct {
	access service.AccessServiceProvider
}

func NewGuard(access service.AccessServiceProvider) *Guard {
	return &Guard{access: access}
}

// Require rejects the request unless the caller holds permission in the scope
// implied by the matched route. The resolved Access is placed on the request
// context so downstream handlers can reuse it without a second lookup.
//
// Register it inline (r.With(guard.Require(...)).Get(...)) rather than with
// r.Use(): inline middlewares run after chi has populated the URL parameters.
func (g *Guard) Require(permission entity.Permission) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			scope, err := g.scope(r)
			if err != nil {
				httputil.WriteInternalServerErrorResponse(w, r, "Failed to resolve access scope", err.Error())
				return
			}

			access, err := g.access.Resolve(r.Context(), scope)
			if err != nil {
				httputil.WriteInternalServerErrorResponse(w, r, "Failed to resolve permissions", err.Error())
				return
			}
			if !access.Can(permission) {
				httputil.WriteForbiddenResponse(w, r, "You do not have permission to perform this action")
				return
			}

			ctx := context.WithValue(r.Context(), accessKey, access)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func (g *Guard) scope(r *http.Request) (service.Scope, error) {
	wsID := chi.URLParam(r, "wsId")
	scope := service.Scope{
		WorkspaceID: wsID,
		UserID:      GetUserIDFromContext(r.Context()),
		Attributes:  GetUserAttributesMapFromContext(r.Context()),
	}

	ctx := r.Context()
	switch {
	case chi.URLParam(r, "projId") != "":
		scope.ProjectID = chi.URLParam(r, "projId")
	case chi.URLParam(r, "dashId") != "":
		projID, err := g.access.ProjectForDashboard(ctx, wsID, chi.URLParam(r, "dashId"))
		if err != nil {
			return scope, err
		}
		scope.ProjectID = projID
	case chi.URLParam(r, "widgetId") != "":
		projID, err := g.access.ProjectForWidget(ctx, wsID, chi.URLParam(r, "widgetId"))
		if err != nil {
			return scope, err
		}
		scope.ProjectID = projID
	}

	return scope, nil
}

// GetAccessFromContext returns the grants resolved by Guard.Require. It is nil
// on routes that are not guarded.
func GetAccessFromContext(ctx context.Context) *service.Access {
	if access, ok := ctx.Value(accessKey).(*service.Access); ok {
		return access
	}
	return nil
}

// GetUserAttributesMapFromContext decodes the JWT attribute claims into the map
// shape RLS resolution expects.
func GetUserAttributesMapFromContext(ctx context.Context) map[string]any {
	raw := GetUserAttributesFromContext(ctx)
	if len(raw) == 0 {
		return nil
	}

	var attributes map[string]any
	if err := json.Unmarshal(raw, &attributes); err != nil {
		return nil
	}
	return attributes
}
