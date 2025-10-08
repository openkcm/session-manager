-- +goose Up
-- +goose StatementBegin

-- Create tables --

CREATE TABLE oidc_providers (
    issuer_url TEXT PRIMARY KEY,
    blocked BOOLEAN NOT NULL DEFAULT FALSE,
    jwks_uris TEXT[] NOT NULL DEFAULT '{}',
    -- aud is usually unique for a tenant; in our case it's always one aud which is CMK. So there's one-to-many relation between tenants and oidc providers.
    audience TEXT[] NOT NULL DEFAULT '{}',
    created_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE TABLE oidc_provider_map (
    tenant_id TEXT PRIMARY KEY,
    issuer_url TEXT NOT NULL
        REFERENCES oidc_providers (issuer_url)
            ON DELETE CASCADE
            ON UPDATE CASCADE
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE oidc_provider_map CASCADE;
DROP TABLE oidc_providers CASCADE;
-- +goose StatementEnd
