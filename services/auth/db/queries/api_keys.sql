-- Generate and store a new API key for a developer
-- name: CreateAPIKey :one
INSERT INTO api_keys (
    user_id,
    name,
    key_hash,
    prefix,
    scopes
) VALUES (
    $1,
    $2,
    $3,
    $4,
    $5
)
RETURNING id, user_id, name, key_hash, prefix, scopes, last_used_at, expires_at, revoked, revoked_at, created_at;

-- Used on every API request to validate keys — must stay minimal
-- name: GetAPIKeyByHash :one
SELECT id, user_id, name, key_hash, prefix, scopes, last_used_at, expires_at, revoked, revoked_at, created_at
FROM api_keys
WHERE key_hash = $1;

-- Fetch a key details securely checking ownership before returning
-- name: GetAPIKeyByID :one
SELECT id, user_id, name, key_hash, prefix, scopes, last_used_at, expires_at, revoked, revoked_at, created_at
FROM api_keys
WHERE id = $1
    AND user_id = $2;

-- List active developer API keys in management panel
-- name: ListAPIKeysByUser :many
SELECT id, user_id, name, key_hash, prefix, scopes, last_used_at, expires_at, revoked, revoked_at, created_at
FROM api_keys
WHERE user_id = $1
    AND revoked = FALSE
ORDER BY created_at DESC;

-- Disable an API key immediately and record revocation timestamp
-- name: RevokeAPIKey :one
UPDATE api_keys
SET revoked = TRUE,
    revoked_at = NOW()
WHERE id = $1
    AND user_id = $2
RETURNING id, user_id, name, key_hash, prefix, scopes, last_used_at, expires_at, revoked, revoked_at, created_at;

-- Minimal update tracking access times without returning data to optimize speed
-- name: UpdateAPIKeyLastUsed :exec
UPDATE api_keys
SET last_used_at = NOW()
WHERE id = $1;

-- Count active API keys for enforcing maximum allowance limit of 10 keys
-- name: CountActiveAPIKeysByUser :one
SELECT count(*)
FROM api_keys
WHERE user_id = $1
    AND revoked = FALSE
    AND (expires_at IS NULL OR expires_at > NOW());

-- Purge keys that were revoked more than 30 days ago to clean database
-- name: DeleteRevokedAPIKeys :exec
DELETE FROM api_keys
WHERE revoked = TRUE
    AND revoked_at < NOW() - INTERVAL '30 days';
