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

CREATE TABLE pkce_state (
    id TEXT PRIMARY KEY,
    tenant_id TEXT NOT NULL,
    fingerprint TEXT NOT NULL,
    verifier TEXT NOT NULL,
    request_uri TEXT NOT NULL,
    expiry TIMESTAMPTZ NOT NULL
);

CREATE TABLE sessions (
    state_id TEXT PRIMARY KEY,
    tenant_id TEXT NOT NULL,
    fingerprint TEXT NOT NULL,
    token TEXT NOT NULL,
    expiry TIMESTAMPTZ NOT NULL
);

-- Create RLS and policies for the tables --
-- TODO: Figure out if we can use RLS on our database. An admin user bypasses RLS and policies, so it might not work with the current setup.

ALTER TABLE oidc_provider_map ENABLE ROW LEVEL SECURITY;
ALTER TABLE oidc_provider_map FORCE ROW LEVEL SECURITY;

CREATE POLICY tenant_map_isolation ON oidc_provider_map
    USING (tenant_id = current_setting('app.tenant_id'));

CREATE POLICY tenant_map_insert ON oidc_provider_map
    FOR INSERT
    WITH CHECK (tenant_id = current_setting('app.tenant_id'));

ALTER TABLE oidc_providers ENABLE ROW LEVEL SECURITY;
ALTER TABLE oidc_providers FORCE ROW LEVEL SECURITY;

CREATE POLICY tenant_provider_insert ON oidc_providers
    FOR INSERT
    WITH CHECK (true);

CREATE POLICY tenant_provider_select ON oidc_providers
    USING (
        EXISTS (
            SELECT true
            FROM oidc_provider_map m
            WHERE m.issuer_url = oidc_providers.issuer_url
                AND m.tenant_id = current_setting('app.tenant_id')
        ));

ALTER TABLE pkce_state ENABLE ROW LEVEL SECURITY;
ALTER TABLE pkce_state FORCE ROW LEVEL SECURITY;

CREATE POLICY pkce_state_insert ON pkce_state
    FOR INSERT
    WITH CHECK (true);

CREATE POLICY pkce_state_select ON pkce_state
    USING (tenant_id = current_setting('app.tenant_id'));
