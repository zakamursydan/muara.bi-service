package service

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
	"muara.bi/core/internal/config"
	"muara.bi/core/internal/domain/entity"
	"muara.bi/core/internal/dto/request"
	"muara.bi/core/internal/dto/response"
	"muara.bi/core/internal/duckdb"
	"muara.bi/core/internal/repository/pgsql"
	"muara.bi/core/pkg/customerror"
	"muara.bi/core/pkg/logging"
)

const (
	shareTokenBytes     = 32
	minSharePasswordLen = 8
	embedTokenTTL       = 12 * time.Hour
	embedTokenIssuer    = "muara.bi/embed"
)

// EmbedClaims is the signed payload an external site presents to render a
// dashboard inside an iframe.
type EmbedClaims struct {
	ShareToken     string   `json:"share_token"`
	DashboardID    string   `json:"dashboard_id"`
	AllowedDomains []string `json:"allowed_domains"`
	EnvTag         string   `json:"env_tag"`
	jwt.RegisteredClaims
}

type ShareServiceProvider interface {
	Create(ctx context.Context, wsID, dashID, userID string, req request.CreateShareRequest) (*response.ShareResponse, error)
	List(ctx context.Context, wsID, dashID string) ([]response.ShareResponse, error)
	Revoke(ctx context.Context, wsID, dashID, shareID string) error
	SignEmbedToken(share *entity.DashboardShare) (string, error)
	VerifyEmbedToken(tokenString string) (*EmbedClaims, error)

	// Public, unauthenticated surface. password is the value of the
	// X-Share-Password header and may be empty for unprotected shares.
	ResolvePublic(ctx context.Context, shareToken, password string) (*response.PublicDashboardResponse, error)
	QueryPublicWidget(ctx context.Context, shareToken, widgetID, password string, req request.PublicWidgetQueryRequest) (*response.ExecuteQueryResponse, error)
}

type ShareService struct {
	shareRepo     pgsql.ShareRepositoryProvider
	dashboardRepo pgsql.DashboardRepositoryProvider
	widgetRepo    pgsql.WidgetRepositoryProvider
	preparer      *SessionPreparer
	cfg           *config.Config
	logger        *logging.Logger
}

func NewShareService(
	shareRepo pgsql.ShareRepositoryProvider,
	dashboardRepo pgsql.DashboardRepositoryProvider,
	widgetRepo pgsql.WidgetRepositoryProvider,
	preparer *SessionPreparer,
	cfg *config.Config,
	logger *logging.Logger,
) *ShareService {
	return &ShareService{
		shareRepo:     shareRepo,
		dashboardRepo: dashboardRepo,
		widgetRepo:    widgetRepo,
		preparer:      preparer,
		cfg:           cfg,
		logger:        logger,
	}
}

func (s *ShareService) Create(ctx context.Context, wsID, dashID, userID string, req request.CreateShareRequest) (*response.ShareResponse, error) {
	dashboard, err := s.dashboardRepo.GetDashboardByID(ctx, wsID, dashID)
	if err != nil {
		return nil, customerror.NewInternalError("failed to load dashboard", err)
	}
	if dashboard == nil {
		return nil, customerror.NewNotFoundError("dashboard not found", nil)
	}

	accessMode := strings.ToUpper(strings.TrimSpace(req.AccessMode))
	switch accessMode {
	case entity.AccessModePublicLink, entity.AccessModeEmbedIFrame:
	default:
		return nil, customerror.NewBadRequestError("access_mode must be PUBLIC_LINK or EMBED_IFRAME", nil)
	}

	domains, err := normalizeAllowedDomains(req.AllowedDomains)
	if err != nil {
		return nil, err
	}
	if accessMode == entity.AccessModeEmbedIFrame && len(domains) == 0 {
		return nil, customerror.NewBadRequestError("EMBED_IFRAME shares require at least one allowed domain", nil)
	}

	if req.ExpiresAt != nil && !req.ExpiresAt.After(time.Now()) {
		return nil, customerror.NewBadRequestError("expires_at must be in the future", nil)
	}

	var passwordHash string
	if password := req.Password; password != "" {
		if len(password) < minSharePasswordLen {
			return nil, customerror.NewBadRequestError(
				fmt.Sprintf("share password must be at least %d characters", minSharePasswordLen), nil)
		}
		hashed, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
		if err != nil {
			return nil, customerror.NewInternalError("failed to hash share password", err)
		}
		passwordHash = string(hashed)
	}

	token, err := generateShareToken()
	if err != nil {
		return nil, err
	}

	share := &entity.DashboardShare{
		DashboardID:    dashID,
		ShareToken:     token,
		AccessMode:     accessMode,
		EnvTag:         NormalizeEnvTag(req.EnvTag),
		PasswordHash:   passwordHash,
		AllowedDomains: domains,
		IsActive:       true,
		ExpiresAt:      req.ExpiresAt,
		CreatedBy:      nilIfEmpty(userID),
	}
	if err := s.shareRepo.CreateShare(ctx, share); err != nil {
		return nil, customerror.NewInternalError("failed to create share", err)
	}
	share.WorkspaceID = wsID

	res := shareResponse(share)
	res.ShareURL = s.publicURL(token)
	if accessMode == entity.AccessModeEmbedIFrame {
		res.EmbedToken, err = s.SignEmbedToken(share)
		if err != nil {
			return nil, err
		}
	}
	return res, nil
}

func (s *ShareService) List(ctx context.Context, wsID, dashID string) ([]response.ShareResponse, error) {
	shares, err := s.shareRepo.ListSharesByDashboard(ctx, wsID, dashID)
	if err != nil {
		return nil, customerror.NewInternalError("failed to list shares", err)
	}

	results := make([]response.ShareResponse, 0, len(shares))
	for i := range shares {
		results = append(results, *shareResponse(&shares[i]))
	}
	return results, nil
}

func (s *ShareService) Revoke(ctx context.Context, wsID, dashID, shareID string) error {
	if err := s.shareRepo.DeleteShare(ctx, wsID, dashID, shareID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return customerror.NewNotFoundError("share not found", nil)
		}
		return customerror.NewInternalError("failed to revoke share", err)
	}
	return nil
}

func (s *ShareService) SignEmbedToken(share *entity.DashboardShare) (string, error) {
	expiresAt := time.Now().Add(embedTokenTTL)
	if share.ExpiresAt != nil && share.ExpiresAt.Before(expiresAt) {
		// Never outlive the share itself.
		expiresAt = *share.ExpiresAt
	}

	claims := EmbedClaims{
		ShareToken:     share.ShareToken,
		DashboardID:    share.DashboardID,
		AllowedDomains: share.AllowedDomains,
		EnvTag:         share.EnvTag,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expiresAt),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Issuer:    embedTokenIssuer,
			Subject:   share.DashboardID,
		},
	}

	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(s.cfg.AppSecretKey))
	if err != nil {
		return "", customerror.NewInternalError("failed to sign embed token", err)
	}
	return token, nil
}

func (s *ShareService) VerifyEmbedToken(tokenString string) (*EmbedClaims, error) {
	claims := &EmbedClaims{}
	token, err := jwt.ParseWithClaims(tokenString, claims, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, jwt.ErrSignatureInvalid
		}
		return []byte(s.cfg.AppSecretKey), nil
	})
	if err != nil || !token.Valid {
		return nil, customerror.NewUnauthorizedError("invalid or expired embed token")
	}
	if claims.Issuer != embedTokenIssuer {
		return nil, customerror.NewUnauthorizedError("invalid embed token issuer")
	}
	return claims, nil
}

// ResolvePublic renders the anonymous canvas payload. A password-protected
// share returns password_required without any dashboard content until the
// viewer supplies the correct password.
func (s *ShareService) ResolvePublic(ctx context.Context, shareToken, password string) (*response.PublicDashboardResponse, error) {
	share, err := s.requireAccessibleShare(ctx, shareToken)
	if err != nil {
		return nil, err
	}

	res := &response.PublicDashboardResponse{
		ShareToken:     share.ShareToken,
		AccessMode:     share.AccessMode,
		FrameAncestors: share.AllowedDomains,
		ExpiresAt:      share.ExpiresAt,
	}

	if share.PasswordHash != "" && password == "" {
		res.PasswordRequired = true
		return res, nil
	}
	if !s.verifySharePassword(share, password) {
		return nil, customerror.NewUnauthorizedError("invalid share password")
	}

	dashboard, err := s.dashboardRepo.GetDashboardByID(ctx, share.WorkspaceID, share.DashboardID)
	if err != nil {
		return nil, customerror.NewInternalError("failed to load shared dashboard", err)
	}
	if dashboard == nil {
		return nil, customerror.NewNotFoundError("shared dashboard no longer exists", nil)
	}

	widgets, err := s.widgetRepo.ListWidgetsByDashboard(ctx, share.DashboardID)
	if err != nil {
		return nil, customerror.NewInternalError("failed to load shared widgets", err)
	}

	res.Title = dashboard.Title
	res.Description = dashboard.Description
	res.LayoutConfig = dashboard.LayoutConfig
	res.GlobalFilters = dashboard.GlobalFilters
	res.Widgets = make([]response.PublicWidgetResponse, 0, len(widgets))
	for _, w := range widgets {
		res.Widgets = append(res.Widgets, response.PublicWidgetResponse{
			ID:            w.ID,
			Title:         w.Title,
			ChartType:     w.ChartType,
			EchartsConfig: w.EchartsConfig,
			MaxRows:       w.MaxRows,
		})
	}
	return res, nil
}

// QueryPublicWidget runs one widget of a shared dashboard. The SQL is read from
// the stored widget row and the environment tag from the share, so an anonymous
// caller can never supply either. Only filter values are accepted, and they are
// bound as parameters.
func (s *ShareService) QueryPublicWidget(
	ctx context.Context,
	shareToken string,
	widgetID string,
	password string,
	req request.PublicWidgetQueryRequest,
) (*response.ExecuteQueryResponse, error) {
	share, err := s.requireAccessibleShare(ctx, shareToken)
	if err != nil {
		return nil, err
	}
	if !s.verifySharePassword(share, password) {
		return nil, customerror.NewUnauthorizedError("invalid share password")
	}

	widget, err := s.widgetRepo.GetWidgetByID(ctx, share.WorkspaceID, widgetID)
	if err != nil {
		return nil, customerror.NewInternalError("failed to load shared widget", err)
	}
	if widget == nil || widget.DashboardID != share.DashboardID {
		return nil, customerror.NewNotFoundError("widget is not part of this shared dashboard", nil)
	}

	execCtx, cancel := duckdb.WithGuardrailTimeout(ctx, 30*time.Second)
	defer cancel()

	result, err := s.preparer.Execute(execCtx, PreparedQuery{
		WorkspaceID: share.WorkspaceID,
		ProjectID:   s.projectIDFor(execCtx, share),
		EnvTag:      share.EnvTag,
		RawSQL:      widget.RawSQL,
		Parameters:  req.Parameters,
		MaxRows:     widget.MaxRows,
	})
	if err != nil {
		return nil, err
	}

	return &response.ExecuteQueryResponse{
		Columns:    result.Columns,
		Rows:       result.Rows,
		RowCount:   result.RowCount,
		DurationMs: result.DurationMs,
		Cached:     false,
		EnvTag:     share.EnvTag,
	}, nil
}

func (s *ShareService) requireAccessibleShare(ctx context.Context, shareToken string) (*entity.DashboardShare, error) {
	token := strings.TrimSpace(shareToken)
	if token == "" {
		return nil, customerror.NewNotFoundError("share link is not available", nil)
	}

	share, err := s.shareRepo.GetShareByToken(ctx, token)
	if err != nil {
		return nil, customerror.NewInternalError("failed to resolve share link", err)
	}
	if share == nil || !share.IsAccessible(time.Now()) {
		return nil, customerror.NewNotFoundError("share link is not available", nil)
	}
	return share, nil
}

// verifySharePassword reports whether the request may read the share. Unprotected
// shares always pass; bcrypt performs the constant-time comparison.
func (s *ShareService) verifySharePassword(share *entity.DashboardShare, password string) bool {
	if share.PasswordHash == "" {
		return true
	}
	if password == "" {
		return false
	}
	return bcrypt.CompareHashAndPassword([]byte(share.PasswordHash), []byte(password)) == nil
}

// projectIDFor resolves the project so semantic models mount for public queries.
func (s *ShareService) projectIDFor(ctx context.Context, share *entity.DashboardShare) string {
	projID, err := s.dashboardRepo.GetProjectIDByDashboard(ctx, share.WorkspaceID, share.DashboardID)
	if err != nil {
		s.logger.Warn(fmt.Sprintf("failed to resolve project for share %s: %v", share.ShareToken, err))
		return ""
	}
	return projID
}

func (s *ShareService) publicURL(token string) string {
	base := strings.TrimSuffix(strings.TrimSpace(s.cfg.AppPublicBaseURL), "/")
	if base == "" {
		return "/share/" + token
	}
	return base + "/share/" + token
}

// generateShareToken returns a 256-bit URL-safe random capability token.
func generateShareToken() (string, error) {
	buf := make([]byte, shareTokenBytes)
	if _, err := rand.Read(buf); err != nil {
		return "", customerror.NewInternalError("failed to generate share token", err)
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

// normalizeAllowedDomains reduces every entry to an absolute scheme://host
// origin. Wildcards, paths, queries and credentials are rejected so the value
// can be emitted verbatim into a frame-ancestors directive. Non-localhost
// origins must use https.
func normalizeAllowedDomains(raw []string) ([]string, error) {
	domains := make([]string, 0, len(raw))
	seen := make(map[string]struct{}, len(raw))

	for _, entry := range raw {
		origin, err := normalizeAllowedDomain(entry)
		if err != nil {
			return nil, err
		}
		if _, dup := seen[origin]; dup {
			continue
		}
		seen[origin] = struct{}{}
		domains = append(domains, origin)
	}
	return domains, nil
}

func normalizeAllowedDomain(raw string) (string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", customerror.NewBadRequestError("allowed domain cannot be empty", nil)
	}

	parsed, err := url.Parse(trimmed)
	if err != nil {
		return "", customerror.NewBadRequestError(fmt.Sprintf("allowed domain %q is not a valid URL", raw), nil)
	}

	invalid := customerror.NewBadRequestError(
		fmt.Sprintf("allowed domain %q must be an absolute origin without path, query or wildcard", raw), nil)

	if parsed.Host == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return "", invalid
	}
	if parsed.Path != "" && parsed.Path != "/" {
		return "", invalid
	}
	if strings.ContainsAny(parsed.Host, "*?") {
		return "", invalid
	}

	hostOnly := parsed.Hostname()
	switch parsed.Scheme {
	case "https":
	case "http":
		if hostOnly != "localhost" && hostOnly != "127.0.0.1" && !strings.HasSuffix(hostOnly, ".localhost") {
			return "", customerror.NewBadRequestError(
				fmt.Sprintf("allowed domain %q must use https", raw), nil)
		}
	default:
		return "", invalid
	}

	return parsed.Scheme + "://" + parsed.Host, nil
}

func shareResponse(share *entity.DashboardShare) *response.ShareResponse {
	domains := share.AllowedDomains
	if domains == nil {
		domains = []string{}
	}
	return &response.ShareResponse{
		ID:                share.ID,
		DashboardID:       share.DashboardID,
		ShareToken:        share.ShareToken,
		AccessMode:        share.AccessMode,
		EnvTag:            share.EnvTag,
		PasswordProtected: share.PasswordHash != "",
		AllowedDomains:    domains,
		IsActive:          share.IsActive,
		ExpiresAt:         share.ExpiresAt,
		CreatedAt:         share.CreatedAt,
	}
}

func nilIfEmpty(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}
