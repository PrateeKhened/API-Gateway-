-- Migration: 000002_create_refresh_tokens
-- Created: auth service
-- Description: Drop refresh_tokens indexes and table

DROP INDEX IF EXISTS idx_refresh_tokens_active_expires_at;

DROP INDEX IF EXISTS idx_refresh_tokens_user_id;

DROP INDEX IF EXISTS idx_refresh_tokens_token_hash;

DROP TABLE IF EXISTS refresh_tokens;
