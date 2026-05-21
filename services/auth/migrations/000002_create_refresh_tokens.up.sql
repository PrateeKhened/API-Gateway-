-- Migration: 000002_create_refresh_tokens
-- Created: auth service
-- Description: Create refresh_tokens table with constraints and indexes

CREATE TABLE IF NOT EXISTS refresh_tokens (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL,
    token_hash TEXT NOT NULL UNIQUE,
    expires_at TIMESTAMPTZ NOT NULL,
    revoked BOOLEAN NOT NULL DEFAULT FALSE,
    revoked_at TIMESTAMPTZ,
    user_agent TEXT,
    ip_address TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT fk_refresh_tokens_user_id FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
    CONSTRAINT chk_refresh_tokens_token_hash_not_empty CHECK (token_hash <> ''),
    CONSTRAINT chk_refresh_tokens_revoked_at CHECK (revoked = FALSE OR revoked_at IS NOT NULL)
);

CREATE INDEX IF NOT EXISTS idx_refresh_tokens_token_hash ON refresh_tokens(token_hash);

CREATE INDEX IF NOT EXISTS idx_refresh_tokens_user_id ON refresh_tokens(user_id);

CREATE INDEX IF NOT EXISTS idx_refresh_tokens_active_expires_at ON refresh_tokens(expires_at) WHERE revoked = FALSE;
