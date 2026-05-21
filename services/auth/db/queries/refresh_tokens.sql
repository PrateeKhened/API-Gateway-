-- Store a newly generated refresh token on login
-- name: CreateRefreshToken :one
INSERT INTO refresh_tokens (
    user_id,
    token_hash,
    expires_at,
    user_agent,
    ip_address
) VALUES (
    $1,
    $2,
    $3,
    $4,
    $5
)
RETURNING id, user_id, token_hash, expires_at, revoked, revoked_at, user_agent, ip_address, created_at;

-- Lookup a token hash to validate session validity and check expiration/revocation
-- name: GetRefreshTokenByHash :one
SELECT id, user_id, token_hash, expires_at, revoked, revoked_at, user_agent, ip_address, created_at
FROM refresh_tokens
WHERE token_hash = $1;

-- Revoke a single refresh token on logout
-- name: RevokeRefreshToken :one
UPDATE refresh_tokens
SET revoked = TRUE,
    revoked_at = NOW()
WHERE id = $1
RETURNING id, user_id, token_hash, expires_at, revoked, revoked_at, user_agent, ip_address, created_at;

-- Revoke all active sessions for a user on password reset or global logout
-- name: RevokeAllUserRefreshTokens :exec
UPDATE refresh_tokens
SET revoked = TRUE,
    revoked_at = NOW()
WHERE user_id = $1
    AND revoked = FALSE;

-- Clean up expired and revoked refresh tokens from database
-- name: DeleteExpiredRefreshTokens :exec
DELETE FROM refresh_tokens
WHERE expires_at < NOW()
    AND revoked = TRUE;

-- List active sessions for security audit dashboard
-- name: ListActiveRefreshTokensByUser :many
SELECT id, user_id, token_hash, expires_at, revoked, revoked_at, user_agent, ip_address, created_at
FROM refresh_tokens
WHERE user_id = $1
    AND revoked = FALSE
    AND expires_at > NOW()
ORDER BY created_at DESC;
