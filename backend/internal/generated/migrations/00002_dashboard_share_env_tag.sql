-- +goose Up
-- Pin the connection environment a public share may query. Without this the
-- anonymous viewer would have to send env_tag itself, which would let anyone
-- holding a share token point the widget query at another environment's
-- credentials.
ALTER TABLE dashboard_shares
    ADD COLUMN IF NOT EXISTS env_tag VARCHAR(50) NOT NULL DEFAULT 'default';

-- +goose Down
ALTER TABLE dashboard_shares DROP COLUMN IF EXISTS env_tag;
