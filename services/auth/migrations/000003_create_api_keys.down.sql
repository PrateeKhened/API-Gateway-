-- Migration: 000003_create_api_keys
-- Created: auth service
-- Description: Drop api_keys indexes and table

DROP INDEX IF EXISTS idx_api_keys_active_expiring;

DROP INDEX IF EXISTS idx_api_keys_prefix;

DROP INDEX IF EXISTS idx_api_keys_user_id;

DROP INDEX IF EXISTS idx_api_keys_key_hash;

DROP TABLE IF EXISTS api_keys;
