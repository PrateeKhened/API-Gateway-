-- Insert a new user on registration
-- name: CreateUser :one
INSERT INTO users (
    email,
    password_hash
) VALUES (
    $1,
    $2
)
RETURNING id, email, password_hash, is_verified, totp_secret, totp_enabled, created_at, updated_at;

-- Retrieve a user by email for authentication checks
-- name: GetUserByEmail :one
SELECT id, email, password_hash, is_verified, totp_secret, totp_enabled, created_at, updated_at
FROM users
WHERE lower(email) = lower($1);

-- Retrieve a user by their unique primary key
-- name: GetUserByID :one
SELECT id, email, password_hash, is_verified, totp_secret, totp_enabled, created_at, updated_at
FROM users
WHERE id = $1;

-- Mark a user's email address as verified
-- name: UpdateUserVerified :one
UPDATE users
SET is_verified = TRUE
WHERE id = $1
RETURNING id, email, password_hash, is_verified, totp_secret, totp_enabled, created_at, updated_at;

-- Enable TOTP 2FA and store the secret atomically
-- name: UpdateTOTPSecret :one
UPDATE users
SET totp_secret = $2,
    totp_enabled = $3
WHERE id = $1
RETURNING id, email, password_hash, is_verified, totp_secret, totp_enabled, created_at, updated_at;

-- Disable TOTP 2FA and remove the stored secret atomically
-- name: DisableTOTP :one
UPDATE users
SET totp_secret = NULL,
    totp_enabled = FALSE
WHERE id = $1
RETURNING id, email, password_hash, is_verified, totp_secret, totp_enabled, created_at, updated_at;

-- Update user password hash on reset or change
-- name: UpdateUserPassword :one
UPDATE users
SET password_hash = $2
WHERE id = $1
RETURNING id, email, password_hash, is_verified, totp_secret, totp_enabled, created_at, updated_at;
