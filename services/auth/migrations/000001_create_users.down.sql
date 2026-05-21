-- Migration: 000001_create_users
-- Created: auth service
-- Description: Drop trigger, set_updated_at function, and users table

DROP TRIGGER IF EXISTS trigger_users_set_updated_at ON users;

DROP FUNCTION IF EXISTS set_updated_at();

DROP TABLE IF EXISTS users;
