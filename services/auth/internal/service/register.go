package service

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"golang.org/x/crypto/bcrypt"

	"github.com/yourusername/devplatform/services/auth/db/generated"
)

var bcryptGenerateFromPassword = bcrypt.GenerateFromPassword


// Register handles user validation, duplicate checks, password hashing,
// and persists the new user to the database.
func (s *Service) Register(ctx context.Context, email, password string) (dbgen.User, error) {
	// Normalize email
	email = strings.ToLower(strings.TrimSpace(email))

	// Validation
	var validationFields []string

	// Email validation
	if email == "" {
		validationFields = append(validationFields, "email: required")
	} else {
		if len(email) < 6 {
			validationFields = append(validationFields, "email: must be at least 6 characters")
		}
		if len(email) > 254 {
			validationFields = append(validationFields, "email: must not exceed 254 characters")
		}
		atIdx := strings.Index(email, "@")
		if atIdx == -1 || !strings.Contains(email[atIdx:], ".") {
			validationFields = append(validationFields, "email: invalid format")
		}
	}

	// Password validation
	if password == "" {
		validationFields = append(validationFields, "password: required")
	} else {
		if len(password) < 8 {
			validationFields = append(validationFields, "password: must be at least 8 characters")
		}
		if len(password) > 72 {
			validationFields = append(validationFields, "password: must not exceed 72 characters")
		}
		hasLetter := false
		hasNumber := false
		for _, r := range password {
			if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') {
				hasLetter = true
			} else if r >= '0' && r <= '9' {
				hasNumber = true
			}
		}
		if !hasLetter || !hasNumber {
			validationFields = append(validationFields, "password: must contain at least one letter and one number")
		}
	}

	if len(validationFields) > 0 {
		return dbgen.User{}, &ValidationError{Fields: validationFields}
	}

	// Duplicate Check
	_, err := s.queries.GetUserByEmail(ctx, email)
	if err == nil {
		// User found -> email already taken
		return dbgen.User{}, ErrEmailTaken
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return dbgen.User{}, fmt.Errorf("register: %w", err)
	}

	// Password Hashing
	hash, err := bcryptGenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return dbgen.User{}, fmt.Errorf("register: %w", err)
	}

	// Create User
	user, err := s.queries.CreateUser(ctx, dbgen.CreateUserParams{
		Email:        email,
		PasswordHash: string(hash),
	})
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return dbgen.User{}, ErrEmailTaken
		}
		return dbgen.User{}, fmt.Errorf("register: %w", err)
	}

	s.logger.Info("user registered", "user_id", formatUUID(user.ID.Bytes), "email", user.Email)

	return user, nil
}

// formatUUID prints a 16-byte UUID array into a standard 8-4-4-4-12 hex string.
func formatUUID(b [16]byte) string {
	return fmt.Sprintf("%02x%02x%02x%02x-%02x%02x-%02x%02x-%02x%02x-%02x%02x%02x%02x%02x%02x",
		b[0], b[1], b[2], b[3],
		b[4], b[5],
		b[6], b[7],
		b[8], b[9],
		b[10], b[11], b[12], b[13], b[14], b[15],
	)
}
