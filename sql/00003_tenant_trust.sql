-- +goose Up
-- +goose StatementBegin
CREATE TABLE trust (
    tenant_id TEXT PRIMARY KEY,
    blocked BOOLEAN NOT NULL DEFAULT FALSE,
    issuer TEXT NOT NULL,
    jwks_uri TEXT NOT NULL DEFAULT '',
    audiences TEXT[] NOT NULL DEFAULT '{}',
    properties JSONB NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT NOW()
);

INSERT INTO trust (tenant_id, blocked, issuer, jwks_uri, audiences, properties, created_at)
    SELECT m.tenant_id, p.blocked, p.issuer_url, COALESCE(p.jwks_uris[1], ''), p.audience, COALESCE(p.properties, '{}'), p.created_at
        FROM oidc_providers p
        INNER JOIN oidc_provider_map m ON p.issuer_url = m.issuer_url;

DROP TABLE oidc_provider_map;
DROP TABLE oidc_providers;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
CREATE TABLE oidc_providers (
    issuer_url TEXT PRIMARY KEY,
    blocked BOOLEAN NOT NULL DEFAULT FALSE,
    jwks_uris TEXT[] NOT NULL DEFAULT '{}',
    audience TEXT[] NOT NULL DEFAULT '{}',
    properties JSONB,
    created_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE TABLE oidc_provider_map (
    tenant_id TEXT PRIMARY KEY,
    issuer_url TEXT NOT NULL
        REFERENCES oidc_providers (issuer_url)
            ON DELETE CASCADE
            ON UPDATE CASCADE
);

INSERT INTO oidc_providers (issuer_url, blocked, jwks_uris, audience, properties, created_at)
    SELECT issuer, blocked, array[jwks_uri], audiences, properties, created_at FROM trust;

INSERT INTO oidc_provider_map (tenant_id, issuer_url)
    SELECT tenant_id, issuer FROM trust;

DROP TABLE trust;
-- +goose StatementEnd
