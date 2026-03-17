-- name: GetOIDCMapping :one
SELECT
    issuer,
    blocked,
    jwks_uri,
    audiences,
    properties,
    client_id
FROM trust
WHERE tenant_id = sqlc.arg(tenant_id);

-- name: CreateOIDCMapping :exec
INSERT INTO trust (
    tenant_id,
    blocked,
    issuer,
    jwks_uri,
    audiences,
    properties,
    client_id)
VALUES (
    sqlc.arg(tenant_id),
    sqlc.arg(blocked),
    sqlc.arg(issuer),
    sqlc.arg(jwks_uri),
    sqlc.arg(audiences),
    sqlc.arg(properties),
    sqlc.arg(client_id));

-- name: DeleteOIDCMapping :execrows
DELETE FROM trust
WHERE tenant_id = sqlc.arg(tenant_id);

-- name: UpdateOIDCMapping :execrows
UPDATE trust
SET
    blocked = sqlc.arg(blocked),
    issuer = sqlc.arg(issuer),
    jwks_uri = sqlc.arg(jwks_uri),
    audiences = sqlc.arg(audiences),
    properties = sqlc.arg(properties),
    client_id = sqlc.arg(client_id)
WHERE
    tenant_id = sqlc.arg(tenant_id);
