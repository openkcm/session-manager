-- +goose Up
-- +goose StatementBegin
ALTER TABLE oidc_providers
    ADD COLUMN IF NOT EXISTS properties JSONB;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE oidc_providers
    DROP COLUMN properties;
-- +goose StatementEnd
