// Package service implements the core business logic domain for the Auth service.
package service

import (
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/yourusername/devplatform/pkg/logger"
	"github.com/yourusername/devplatform/services/auth/db/generated"
)

// Sentinel errors representing common business logic failures.
var (
	ErrEmailTaken   = errors.New("email already taken")
	ErrValidation   = errors.New("validation failed")
	ErrNotFound     = errors.New("not found")
	ErrUnauthorized = errors.New("unauthorized")
)

// ValidationError represents field-level user input validation errors.
type ValidationError struct {
	Fields []string
}

// Error implements the standard Go error interface.
func (e *ValidationError) Error() string {
	return fmt.Sprintf("validation failed: %v", e.Fields)
}

// Unwrap returns the root ErrValidation to allow errors.Is checks.
func (e *ValidationError) Unwrap() error {
	return ErrValidation
}

// Service manages the business workflows of the Auth service.
type Service struct {
	db      *pgxpool.Pool
	queries dbgen.Querier
	logger  *logger.Logger
}

// NewService creates a new Service instance.
func NewService(db *pgxpool.Pool, log *logger.Logger) *Service {
	return &Service{
		db:      db,
		queries: dbgen.New(db),
		logger:  log,
	}
}
