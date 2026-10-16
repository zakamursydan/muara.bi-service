package pgsql

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"muara.bi/core/internal/domain/entity"
)

type ShareRepositoryProvider interface {
	CreateShare(ctx context.Context, share *entity.DashboardShare) error
	ListSharesByDashboard(ctx context.Context, workspaceID, dashboardID string) ([]entity.DashboardShare, error)
	GetShareByID(ctx context.Context, workspaceID, shareID string) (*entity.DashboardShare, error)
	GetShareByToken(ctx context.Context, shareToken string) (*entity.DashboardShare, error)
	DeleteShare(ctx context.Context, workspaceID, dashboardID, shareID string) error
}

type ShareRepository struct {
	db *sql.DB
}

func NewShareRepository(db *sql.DB) *ShareRepository {
	return &ShareRepository{db: db}
}

// shareColumns joins dashboards so every lookup stays scoped to one tenant.
const shareColumns = `s.id, s.dashboard_id, s.share_token, s.access_mode, s.env_tag, s.password_hash,
	s.allowed_domains, s.is_active, s.expires_at, s.created_by, s.created_at, d.workspace_id`

func (r *ShareRepository) CreateShare(ctx context.Context, share *entity.DashboardShare) error {
	const query = `
		INSERT INTO dashboard_shares (dashboard_id, share_token, access_mode, env_tag, password_hash, allowed_domains, is_active, expires_at, created_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING id, created_at`

	domains, err := json.Marshal(share.AllowedDomains)
	if err != nil {
		return fmt.Errorf("failed to serialize allowed domains: %w", err)
	}

	return r.db.QueryRowContext(ctx, query,
		share.DashboardID, share.ShareToken, share.AccessMode, share.EnvTag,
		nullableString(share.PasswordHash), domains, share.IsActive, share.ExpiresAt, share.CreatedBy,
	).Scan(&share.ID, &share.CreatedAt)
}

func (r *ShareRepository) ListSharesByDashboard(ctx context.Context, workspaceID, dashboardID string) ([]entity.DashboardShare, error) {
	query := fmt.Sprintf(`
		SELECT %s
		FROM dashboard_shares s
		JOIN dashboards d ON d.id = s.dashboard_id
		WHERE d.workspace_id = $1 AND s.dashboard_id = $2
		ORDER BY s.created_at DESC`, shareColumns)

	rows, err := r.db.QueryContext(ctx, query, workspaceID, dashboardID)
	if err != nil {
		return nil, fmt.Errorf("failed to list shares: %w", err)
	}
	defer rows.Close()

	var shares []entity.DashboardShare
	for rows.Next() {
		share, err := scanShare(rows)
		if err != nil {
			return nil, err
		}
		shares = append(shares, *share)
	}
	return shares, rows.Err()
}

func (r *ShareRepository) GetShareByID(ctx context.Context, workspaceID, shareID string) (*entity.DashboardShare, error) {
	query := fmt.Sprintf(`
		SELECT %s
		FROM dashboard_shares s
		JOIN dashboards d ON d.id = s.dashboard_id
		WHERE d.workspace_id = $1 AND s.id = $2`, shareColumns)

	share, err := scanShare(r.db.QueryRowContext(ctx, query, workspaceID, shareID))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return share, nil
}

// GetShareByToken resolves a public share by its capability token. There is no
// workspace filter: possession of the token is the only credential, so callers
// must still enforce is_active, expiry, password and allowed_domains.
func (r *ShareRepository) GetShareByToken(ctx context.Context, shareToken string) (*entity.DashboardShare, error) {
	query := fmt.Sprintf(`
		SELECT %s
		FROM dashboard_shares s
		JOIN dashboards d ON d.id = s.dashboard_id
		WHERE s.share_token = $1`, shareColumns)

	share, err := scanShare(r.db.QueryRowContext(ctx, query, shareToken))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return share, nil
}

func (r *ShareRepository) DeleteShare(ctx context.Context, workspaceID, dashboardID, shareID string) error {
	const query = `
		DELETE FROM dashboard_shares s
		USING dashboards d
		WHERE d.id = s.dashboard_id AND d.workspace_id = $1 AND s.dashboard_id = $2 AND s.id = $3`

	res, err := r.db.ExecContext(ctx, query, workspaceID, dashboardID, shareID)
	if err != nil {
		return fmt.Errorf("failed to revoke share: %w", err)
	}
	if affected, err := res.RowsAffected(); err == nil && affected == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func scanShare(s rowScanner) (*entity.DashboardShare, error) {
	var share entity.DashboardShare
	var passwordHash sql.NullString
	var domains []byte

	if err := s.Scan(
		&share.ID, &share.DashboardID, &share.ShareToken, &share.AccessMode, &share.EnvTag,
		&passwordHash, &domains, &share.IsActive, &share.ExpiresAt, &share.CreatedBy,
		&share.CreatedAt, &share.WorkspaceID,
	); err != nil {
		return nil, fmt.Errorf("failed to scan share: %w", err)
	}

	share.PasswordHash = passwordHash.String
	parsed, err := parseStringArray(domains)
	if err != nil {
		return nil, err
	}
	share.AllowedDomains = parsed

	return &share, nil
}

func nullableString(value string) any {
	if value == "" {
		return nil
	}
	return value
}

func parseStringArray(raw []byte) ([]string, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return []string{}, nil
	}

	var values []string
	if err := json.Unmarshal(raw, &values); err != nil {
		return nil, fmt.Errorf("failed to parse allowed domains: %w", err)
	}
	if values == nil {
		return []string{}, nil
	}
	return values, nil
}
