-- Migration: 000003_create_api_keys
-- Created: auth service
-- Description: Create api_keys table with constraints and indexes

CREATE TABLE IF NOT EXISTS api_keys (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL,
    name TEXT NOT NULL,
    key_hash TEXT NOT NULL UNIQUE,
    prefix TEXT NOT NULL,
    scopes TEXT[] NOT NULL DEFAULT '{}',
    last_used_at TIMESTAMPTZ,
    expires_at TIMESTAMPTZ,
    revoked BOOLEAN NOT NULL DEFAULT FALSE,
    revoked_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT fk_api_keys_user_id FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
    CONSTRAINT chk_api_keys_key_hash_not_empty CHECK (key_hash <> ''),
    CONSTRAINT chk_api_keys_prefix_not_empty CHECK (prefix <> ''),
    CONSTRAINT chk_api_keys_name_not_empty CHECK (name <> ''),
    CONSTRAINT chk_api_keys_name_length CHECK (char_length(name) <= 100),
    CONSTRAINT chk_api_keys_revoked_at CHECK (revoked = FALSE OR revoked_at IS NOT NULL)
);

CREATE INDEX IF NOT EXISTS idx_api_keys_key_hash ON api_keys(key_hash);

CREATE INDEX IF NOT EXISTS idx_api_keys_user_id ON api_keys(user_id);

CREATE INDEX IF NOT EXISTS idx_api_keys_prefix ON api_keys(prefix);

CREATE INDEX IF NOT EXISTS idx_api_keys_active_expiring ON api_keys(expires_at) WHERE revoked = FALSE AND expires_at IS NOT NULL;
