-- +goose Up
ALTER TABLE oidc_providers
    ADD COLUMN IF NOT EXISTS properties JSONB;
