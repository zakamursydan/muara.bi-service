package handler

import (
	"net/http"
	"net/url"
	"strings"

	"github.com/go-chi/chi/v5"
	"muara.bi/core/internal/domain/entity"
	"muara.bi/core/internal/dto/request"
	"muara.bi/core/internal/dto/response"
	"muara.bi/core/internal/service"
	"muara.bi/core/pkg/httputil"
)

// sharePasswordHeader carries the viewer's password for a protected share. It
// travels in a header rather than the body so the same mechanism covers the GET
// that renders the canvas.
const sharePasswordHeader = "X-Share-Password"

// PublicHandler serves the unauthenticated share surface. Paths are relative to
// the /api/v1/public router; none of them accept SQL, an environment tag or a
// tenant identifier from the caller.
type PublicHandler struct {
	service   service.ShareServiceProvider
	appOrigin string
}

func NewPublicHandler(service service.ShareServiceProvider, appPublicBaseURL string) *PublicHandler {
	return &PublicHandler{service: service, appOrigin: originOf(appPublicBaseURL)}
}

func (h *PublicHandler) RegisterRoutes(r chi.Router) {
	r.Get("/dashboards/{shareToken}", h.Dashboard)
	r.Post("/dashboards/{shareToken}/widgets/{widgetId}/query", h.QueryWidget)
	r.Get("/embed", h.Embed)
}

func (h *PublicHandler) Dashboard(w http.ResponseWriter, r *http.Request) {
	shareToken := chi.URLParam(r, "shareToken")

	res, err := h.service.ResolvePublic(r.Context(), shareToken, r.Header.Get(sharePasswordHeader))
	if err != nil {
		WriteServiceError(w, r, err)
		return
	}

	writeFrameAncestors(w, res)
	httputil.WriteSuccessResponse(w, r, "Shared dashboard fetched successfully", res)
}

// QueryWidget executes one stored widget of a shared dashboard.
func (h *PublicHandler) QueryWidget(w http.ResponseWriter, r *http.Request) {
	shareToken := chi.URLParam(r, "shareToken")
	widgetID := chi.URLParam(r, "widgetId")

	var req request.PublicWidgetQueryRequest
	if !DecodeJSON(w, r, &req) {
		return
	}

	res, err := h.service.QueryPublicWidget(r.Context(), shareToken, widgetID, r.Header.Get(sharePasswordHeader), req)
	if err != nil {
		WriteServiceError(w, r, err)
		return
	}

	httputil.WriteSuccessResponse(w, r, "Widget data fetched successfully", res)
}

// Embed exchanges a signed embed JWT for the same canvas payload as the public
// link, so an external site never has to expose the raw share token in markup.
func (h *PublicHandler) Embed(w http.ResponseWriter, r *http.Request) {
	claims, err := h.service.VerifyEmbedToken(r.URL.Query().Get("token"))
	if err != nil {
		WriteServiceError(w, r, err)
		return
	}

	res, err := h.service.ResolvePublic(r.Context(), claims.ShareToken, r.Header.Get(sharePasswordHeader))
	if err != nil {
		WriteServiceError(w, r, err)
		return
	}
	if res.AccessMode != entity.AccessModeEmbedIFrame {
		httputil.WriteForbiddenResponse(w, r, "This share is not configured for iframe embedding")
		return
	}

	// Defence in depth: the browser enforces frame-ancestors, but a cross-site
	// fetch issued straight from a non-whitelisted origin is rejected here too.
	if !embedOriginAllowed(r, h.appOrigin, claims.AllowedDomains) {
		httputil.WriteForbiddenResponse(w, r, "This origin is not allowed to embed the dashboard")
		return
	}

	writeFrameAncestors(w, res)
	httputil.WriteSuccessResponse(w, r, "Embedded dashboard fetched successfully", res)
}

// embedOriginAllowed reports whether the browser-supplied origin may embed the
// dashboard. A request without an origin (server-to-server, or a navigation with
// a strict referrer policy) is left to the CSP header.
func embedOriginAllowed(r *http.Request, appOrigin string, allowed []string) bool {
	origin := originOf(r.Header.Get("Origin"))
	if origin == "" {
		origin = originOf(r.Header.Get("Referer"))
	}
	if origin == "" {
		return true
	}
	if appOrigin != "" && strings.EqualFold(appOrigin, origin) {
		return true
	}
	for _, candidate := range allowed {
		if strings.EqualFold(candidate, origin) {
			return true
		}
	}
	return false
}

// writeFrameAncestors constrains who may frame the payload. Shares without an
// explicit allowlist deny framing entirely.
func writeFrameAncestors(w http.ResponseWriter, res *response.PublicDashboardResponse) {
	if len(res.FrameAncestors) == 0 {
		w.Header().Set("Content-Security-Policy", "frame-ancestors 'none'")
		w.Header().Set("X-Frame-Options", "DENY")
		return
	}
	// X-Frame-Options has no allowlist syntax, so it is omitted here; browsers
	// that support CSP honour frame-ancestors instead.
	w.Header().Set("Content-Security-Policy", "frame-ancestors "+strings.Join(res.FrameAncestors, " "))
}

// originOf reduces a URL to its scheme://host, returning "" when the value is
// not an absolute URL.
func originOf(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return ""
	}
	parsed, err := url.Parse(trimmed)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return ""
	}
	return parsed.Scheme + "://" + parsed.Host
}
