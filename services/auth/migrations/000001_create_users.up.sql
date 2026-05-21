-- Migration: 000001_create_users
-- Created: auth service
-- Description: Enable pgcrypto, create users table with constraints and updated_at trigger

CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE OR REPLACE FUNCTION set_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TABLE IF NOT EXISTS users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email TEXT NOT NULL UNIQUE,
    password_hash TEXT NOT NULL,
    is_verified BOOLEAN NOT NULL DEFAULT FALSE,
    totp_secret TEXT,
    totp_enabled BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT chk_users_email_lowercase CHECK (email = lower(email)),
    CONSTRAINT chk_users_email_format CHECK (email LIKE '%@%'),
    CONSTRAINT chk_users_password_hash_not_empty CHECK (password_hash <> ''),
    CONSTRAINT chk_users_totp_secret CHECK (totp_enabled = FALSE OR totp_secret IS NOT NULL)
);

DROP TRIGGER IF EXISTS trigger_users_set_updated_at ON users;

CREATE TRIGGER trigger_users_set_updated_at
BEFORE UPDATE ON users
FOR EACH ROW
EXECUTE FUNCTION set_updated_at();
