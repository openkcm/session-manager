-- +goose Up
ALTER TABLE oidc_providers
    ADD COLUMN properties JSONB;

-- +goose Down
ALTER TABLE oidc_providers
DROP COLUMN IF EXISTS properties;