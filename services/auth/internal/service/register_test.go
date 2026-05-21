package service

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/yourusername/devplatform/pkg/logger"
	"github.com/yourusername/devplatform/services/auth/db/generated"
)

type mockQuerier struct {
	dbgen.Querier
	GetUserByEmailFunc func(ctx context.Context, email string) (dbgen.User, error)
	CreateUserFunc     func(ctx context.Context, arg dbgen.CreateUserParams) (dbgen.User, error)
}

func (m *mockQuerier) GetUserByEmail(ctx context.Context, email string) (dbgen.User, error) {
	if m.GetUserByEmailFunc != nil {
		return m.GetUserByEmailFunc(ctx, email)
	}
	return dbgen.User{}, pgx.ErrNoRows
}

func (m *mockQuerier) CreateUser(ctx context.Context, arg dbgen.CreateUserParams) (dbgen.User, error) {
	if m.CreateUserFunc != nil {
		return m.CreateUserFunc(ctx, arg)
	}
	return dbgen.User{}, nil
}

func TestRegister(t *testing.T) {
	// Restore standard bcrypt generator after tests
	originalBcrypt := bcryptGenerateFromPassword
	defer func() {
		bcryptGenerateFromPassword = originalBcrypt
	}()

	tests := []struct {
		name          string
		email         string
		password      string
		mockGetEmail  func(ctx context.Context, email string) (dbgen.User, error)
		mockCreate    func(ctx context.Context, arg dbgen.CreateUserParams) (dbgen.User, error)
		mockBcrypt    func(password []byte, cost int) ([]byte, error)
		expectErr     error
		verifyUser    func(t *testing.T, u dbgen.User)
		expectValErr  bool
	}{
		{
			name:     "valid registration",
			email:    "test@example.com",
			password: "password123",
			mockGetEmail: func(ctx context.Context, email string) (dbgen.User, error) {
				return dbgen.User{}, pgx.ErrNoRows
			},
			mockCreate: func(ctx context.Context, arg dbgen.CreateUserParams) (dbgen.User, error) {
				if arg.Email != "test@example.com" {
					t.Errorf("expected email test@example.com, got %s", arg.Email)
				}
				return dbgen.User{
					Email:      arg.Email,
					IsVerified: false,
				}, nil
			},
			expectErr: nil,
			verifyUser: func(t *testing.T, u dbgen.User) {
				if u.Email != "test@example.com" {
					t.Errorf("expected email test@example.com, got %s", u.Email)
				}
				if u.IsVerified {
					t.Error("expected is_verified to be false")
				}
			},
		},
		{
			name:     "normalization to lowercase",
			email:    "TEST@EXAMPLE.com",
			password: "password123",
			mockGetEmail: func(ctx context.Context, email string) (dbgen.User, error) {
				if email != "test@example.com" {
					t.Errorf("expected email to be lowercased to test@example.com, got %s", email)
				}
				return dbgen.User{}, pgx.ErrNoRows
			},
			mockCreate: func(ctx context.Context, arg dbgen.CreateUserParams) (dbgen.User, error) {
				if arg.Email != "test@example.com" {
					t.Errorf("expected created email to be test@example.com, got %s", arg.Email)
				}
				return dbgen.User{
					Email: arg.Email,
				}, nil
			},
			expectErr: nil,
		},
		{
			name:     "duplicate email check at app level",
			email:    "duplicate@example.com",
			password: "password123",
			mockGetEmail: func(ctx context.Context, email string) (dbgen.User, error) {
				return dbgen.User{Email: email}, nil
			},
			expectErr: ErrEmailTaken,
		},
		{
			name:     "duplicate email check at DB level (race condition 23505)",
			email:    "duplicate@example.com",
			password: "password123",
			mockGetEmail: func(ctx context.Context, email string) (dbgen.User, error) {
				return dbgen.User{}, pgx.ErrNoRows
			},
			mockCreate: func(ctx context.Context, arg dbgen.CreateUserParams) (dbgen.User, error) {
				return dbgen.User{}, &pgconn.PgError{Code: "23505"}
			},
			expectErr: ErrEmailTaken,
		},
		{
			name:         "empty email",
			email:        "",
			password:     "password123",
			expectValErr: true,
		},
		{
			name:         "email too short",
			email:        "a@b.c",
			password:     "password123",
			expectValErr: true,
		},
		{
			name:         "email too long",
			email:        strings.Repeat("a", 250) + "@example.com",
			password:     "password123",
			expectValErr: true,
		},
		{
			name:         "email missing @ and dot",
			email:        "invalidemail",
			password:     "password123",
			expectValErr: true,
		},
		{
			name:         "empty password",
			email:        "test@example.com",
			password:     "",
			expectValErr: true,
		},
		{
			name:         "password too short",
			email:        "test@example.com",
			password:     "pass1",
			expectValErr: true,
		},
		{
			name:         "password too long",
			email:        "test@example.com",
			password:     strings.Repeat("a", 73),
			expectValErr: true,
		},
		{
			name:         "password with no numbers",
			email:        "test@example.com",
			password:     "password",
			expectValErr: true,
		},
		{
			name:     "unexpected DB error on GetUserByEmail",
			email:    "test@example.com",
			password: "password123",
			mockGetEmail: func(ctx context.Context, email string) (dbgen.User, error) {
				return dbgen.User{}, errors.New("connection failed")
			},
			expectErr: errors.New("connection failed"),
		},
		{
			name:     "bcrypt hashing failure mock",
			email:    "test@example.com",
			password: "password123",
			mockGetEmail: func(ctx context.Context, email string) (dbgen.User, error) {
				return dbgen.User{}, pgx.ErrNoRows
			},
			mockBcrypt: func(password []byte, cost int) ([]byte, error) {
				return nil, errors.New("hashing failed")
			},
			expectErr: errors.New("hashing failed"),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			mock := &mockQuerier{
				GetUserByEmailFunc: tc.mockGetEmail,
				CreateUserFunc:     tc.mockCreate,
			}

			// Swap bcrypt generator if mock is provided
			if tc.mockBcrypt != nil {
				bcryptGenerateFromPassword = tc.mockBcrypt
			} else {
				bcryptGenerateFromPassword = originalBcrypt
			}

			svc := &Service{
				queries: mock,
				logger:  logger.NewNopLogger(),
			}

			user, err := svc.Register(context.Background(), tc.email, tc.password)

			if tc.expectValErr {
				if err == nil {
					t.Fatal("expected validation error, got nil")
				}
				if !errors.Is(err, ErrValidation) {
					t.Errorf("expected error to wrap ErrValidation, got %v", err)
				}
				var valErr *ValidationError
				if !errors.As(err, &valErr) {
					t.Fatalf("expected ValidationError type, got %T", err)
				}
				if len(valErr.Fields) == 0 {
					t.Error("expected non-empty ValidationError Fields slice")
				}
				return
			}

			if tc.expectErr != nil {
				if err == nil {
					t.Fatalf("expected error containing %v, got nil", tc.expectErr)
				}
				if !errors.Is(err, tc.expectErr) && !strings.Contains(err.Error(), tc.expectErr.Error()) {
					t.Errorf("expected error %v, got %v", tc.expectErr, err)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if tc.verifyUser != nil {
				tc.verifyUser(t, user)
			}
		})
	}
}
