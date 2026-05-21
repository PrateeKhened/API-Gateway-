package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/yourusername/devplatform/pkg/logger"
	"github.com/yourusername/devplatform/services/auth/db/generated"
	"github.com/yourusername/devplatform/services/auth/internal/service"
)

type registrar interface {
	Register(ctx context.Context, email, password string) (dbgen.User, error)
}

type mockRegistrar struct {
	registerFunc func(ctx context.Context, email, password string) (dbgen.User, error)
}

func (m *mockRegistrar) Register(ctx context.Context, email, password string) (dbgen.User, error) {
	if m.registerFunc != nil {
		return m.registerFunc(ctx, email, password)
	}
	return dbgen.User{}, nil
}

type errorResponse struct {
	Error struct {
		Message   string   `json:"message"`
		Code      string   `json:"code"`
		RequestID string   `json:"request_id"`
		Details   []string `json:"details"`
	} `json:"error"`
}

func TestRegisterHandler(t *testing.T) {
	tests := []struct {
		name           string
		requestBody    string
		mockRegister   func(ctx context.Context, email, password string) (dbgen.User, error)
		expectedStatus int
		verifyBody     func(t *testing.T, body []byte)
	}{
		{
			name:        "valid input",
			requestBody: `{"email":"rahul@example.com","password":"supersecret123"}`,
			mockRegister: func(ctx context.Context, email, password string) (dbgen.User, error) {
				return dbgen.User{
					ID:         pgtype.UUID{Bytes: [16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}, Valid: true},
					Email:      email,
					IsVerified: false,
					CreatedAt:  pgtype.Timestamptz{Time: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), Valid: true},
				}, nil
			},
			expectedStatus: http.StatusCreated,
			verifyBody: func(t *testing.T, body []byte) {
				var resp RegisterResponse
				if err := json.Unmarshal(body, &resp); err != nil {
					t.Fatalf("failed to unmarshal success body: %v", err)
				}
				if resp.User.Email != "rahul@example.com" {
					t.Errorf("expected email rahul@example.com, got %s", resp.User.Email)
				}
				if resp.User.ID != "01020304-0506-0708-090a-0b0c0d0e0f10" {
					t.Errorf("expected formatted UUID, got %s", resp.User.ID)
				}
				if resp.User.IsVerified {
					t.Error("expected is_verified to be false")
				}
				if resp.User.CreatedAt != "2026-01-01T00:00:00Z" {
					t.Errorf("expected created_at 2026-01-01T00:00:00Z, got %s", resp.User.CreatedAt)
				}
			},
		},
		{
			name:           "empty body",
			requestBody:    "",
			expectedStatus: http.StatusBadRequest,
			verifyBody: func(t *testing.T, body []byte) {
				var resp errorResponse
				if err := json.Unmarshal(body, &resp); err != nil {
					t.Fatalf("failed to unmarshal error body: %v", err)
				}
				if resp.Error.Code != "INVALID_INPUT" {
					t.Errorf("expected ErrorCode INVALID_INPUT, got %s", resp.Error.Code)
				}
				if !strings.Contains(resp.Error.Message, "body is required") {
					t.Errorf("expected error message context, got %s", resp.Error.Message)
				}
			},
		},
		{
			name:        "missing email field",
			requestBody: `{"password":"supersecret123"}`,
			mockRegister: func(ctx context.Context, email, password string) (dbgen.User, error) {
				return dbgen.User{}, &service.ValidationError{Fields: []string{"email: required"}}
			},
			expectedStatus: http.StatusUnprocessableEntity,
			verifyBody: func(t *testing.T, body []byte) {
				var resp errorResponse
				if err := json.Unmarshal(body, &resp); err != nil {
					t.Fatalf("failed to unmarshal error body: %v", err)
				}
				if resp.Error.Code != "UNPROCESSABLE_ENTITY" {
					t.Errorf("expected ErrorCode UNPROCESSABLE_ENTITY, got %s", resp.Error.Code)
				}
				if len(resp.Error.Details) != 1 || resp.Error.Details[0] != "email: required" {
					t.Errorf("expected details with email: required, got %v", resp.Error.Details)
				}
			},
		},
		{
			name:        "missing password field",
			requestBody: `{"email":"rahul@example.com"}`,
			mockRegister: func(ctx context.Context, email, password string) (dbgen.User, error) {
				return dbgen.User{}, &service.ValidationError{Fields: []string{"password: required"}}
			},
			expectedStatus: http.StatusUnprocessableEntity,
			verifyBody: func(t *testing.T, body []byte) {
				var resp errorResponse
				if err := json.Unmarshal(body, &resp); err != nil {
					t.Fatalf("failed to unmarshal error body: %v", err)
				}
				if resp.Error.Code != "UNPROCESSABLE_ENTITY" {
					t.Errorf("expected ErrorCode UNPROCESSABLE_ENTITY, got %s", resp.Error.Code)
				}
				if len(resp.Error.Details) != 1 || resp.Error.Details[0] != "password: required" {
					t.Errorf("expected details with password: required, got %v", resp.Error.Details)
				}
			},
		},
		{
			name:        "password too short",
			requestBody: `{"email":"rahul@example.com","password":"short"}`,
			mockRegister: func(ctx context.Context, email, password string) (dbgen.User, error) {
				return dbgen.User{}, &service.ValidationError{Fields: []string{"password: must be at least 8 characters"}}
			},
			expectedStatus: http.StatusUnprocessableEntity,
			verifyBody: func(t *testing.T, body []byte) {
				var resp errorResponse
				if err := json.Unmarshal(body, &resp); err != nil {
					t.Fatalf("failed to unmarshal error body: %v", err)
				}
				if resp.Error.Code != "UNPROCESSABLE_ENTITY" {
					t.Errorf("expected ErrorCode UNPROCESSABLE_ENTITY, got %s", resp.Error.Code)
				}
				if len(resp.Error.Details) != 1 || !strings.Contains(resp.Error.Details[0], "must be at least 8 characters") {
					t.Errorf("expected details containing too short limit, got %v", resp.Error.Details)
				}
			},
		},
		{
			name:        "password too long",
			requestBody: `{"email":"rahul@example.com","password":"` + strings.Repeat("a", 75) + `"}`,
			mockRegister: func(ctx context.Context, email, password string) (dbgen.User, error) {
				return dbgen.User{}, &service.ValidationError{Fields: []string{"password: must not exceed 72 characters"}}
			},
			expectedStatus: http.StatusUnprocessableEntity,
			verifyBody: func(t *testing.T, body []byte) {
				var resp errorResponse
				if err := json.Unmarshal(body, &resp); err != nil {
					t.Fatalf("failed to unmarshal error body: %v", err)
				}
				if resp.Error.Code != "UNPROCESSABLE_ENTITY" {
					t.Errorf("expected ErrorCode UNPROCESSABLE_ENTITY, got %s", resp.Error.Code)
				}
				if len(resp.Error.Details) != 1 || !strings.Contains(resp.Error.Details[0], "must not exceed 72 characters") {
					t.Errorf("expected details containing max limit, got %v", resp.Error.Details)
				}
			},
		},
		{
			name:        "email already taken",
			requestBody: `{"email":"duplicate@example.com","password":"supersecret123"}`,
			mockRegister: func(ctx context.Context, email, password string) (dbgen.User, error) {
				return dbgen.User{}, service.ErrEmailTaken
			},
			expectedStatus: http.StatusConflict,
			verifyBody: func(t *testing.T, body []byte) {
				var resp errorResponse
				if err := json.Unmarshal(body, &resp); err != nil {
					t.Fatalf("failed to unmarshal error body: %v", err)
				}
				if resp.Error.Code != "EMAIL_ALREADY_EXISTS" {
					t.Errorf("expected ErrorCode EMAIL_ALREADY_EXISTS, got %s", resp.Error.Code)
				}
				if !strings.Contains(resp.Error.Message, "already registered") {
					t.Errorf("expected conflict message context, got %s", resp.Error.Message)
				}
			},
		},
		{
			name:           "invalid JSON",
			requestBody:    `{"email": "rahul@example.com", "password":`,
			expectedStatus: http.StatusBadRequest,
			verifyBody: func(t *testing.T, body []byte) {
				var resp errorResponse
				if err := json.Unmarshal(body, &resp); err != nil {
					t.Fatalf("failed to unmarshal error body: %v", err)
				}
				if resp.Error.Code != "INVALID_INPUT" {
					t.Errorf("expected ErrorCode INVALID_INPUT, got %s", resp.Error.Code)
				}
				if !strings.Contains(resp.Error.Message, "syntax error") && !strings.Contains(resp.Error.Message, "malformed JSON") && !strings.Contains(resp.Error.Message, "unexpected end of input") {
					t.Errorf("expected syntax error or malformed JSON message, got %s", resp.Error.Message)
				}
			},
		},
		{
			name:        "service returns unexpected error",
			requestBody: `{"email":"rahul@example.com","password":"supersecret123"}`,
			mockRegister: func(ctx context.Context, email, password string) (dbgen.User, error) {
				return dbgen.User{}, errors.New("db failure")
			},
			expectedStatus: http.StatusInternalServerError,
			verifyBody: func(t *testing.T, body []byte) {
				var resp errorResponse
				if err := json.Unmarshal(body, &resp); err != nil {
					t.Fatalf("failed to unmarshal error body: %v", err)
				}
				if resp.Error.Code != "INTERNAL_ERROR" {
					t.Errorf("expected ErrorCode INTERNAL_ERROR, got %s", resp.Error.Code)
				}
				if strings.Contains(resp.Error.Message, "db failure") {
					t.Error("error response must suppress internal database failure message")
				}
			},
		},
		{
			name:        "email with uppercase letters",
			requestBody: `{"email":"RAHUL@EXAMPLE.com","password":"supersecret123"}`,
			mockRegister: func(ctx context.Context, email, password string) (dbgen.User, error) {
				return dbgen.User{
					ID:         pgtype.UUID{Bytes: [16]byte{1}, Valid: true},
					Email:      strings.ToLower(email),
					IsVerified: false,
					CreatedAt:  pgtype.Timestamptz{Time: time.Now(), Valid: true},
				}, nil
			},
			expectedStatus: http.StatusCreated,
			verifyBody: func(t *testing.T, body []byte) {
				var resp RegisterResponse
				if err := json.Unmarshal(body, &resp); err != nil {
					t.Fatalf("failed to unmarshal success body: %v", err)
				}
				if resp.User.Email != "rahul@example.com" {
					t.Errorf("expected lowercased email rahul@example.com, got %s", resp.User.Email)
				}
			},
		},
		{
			name:        "password without numbers",
			requestBody: `{"email":"rahul@example.com","password":"password"}`,
			mockRegister: func(ctx context.Context, email, password string) (dbgen.User, error) {
				return dbgen.User{}, &service.ValidationError{Fields: []string{"password: must contain at least one letter and one number"}}
			},
			expectedStatus: http.StatusUnprocessableEntity,
			verifyBody: func(t *testing.T, body []byte) {
				var resp errorResponse
				if err := json.Unmarshal(body, &resp); err != nil {
					t.Fatalf("failed to unmarshal error body: %v", err)
				}
				if resp.Error.Code != "UNPROCESSABLE_ENTITY" {
					t.Errorf("expected ErrorCode UNPROCESSABLE_ENTITY, got %s", resp.Error.Code)
				}
				if len(resp.Error.Details) != 1 || !strings.Contains(resp.Error.Details[0], "at least one letter and one number") {
					t.Errorf("expected complexity message, got %v", resp.Error.Details)
				}
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			mock := &mockRegistrar{
				registerFunc: tc.mockRegister,
			}

			h := &Handler{
				svc:    mock,
				logger: logger.NewNopLogger(),
			}

			w := httptest.NewRecorder()
			r := httptest.NewRequest(http.MethodPost, "/auth/register", bytes.NewBufferString(tc.requestBody))
			r.Header.Set("Content-Type", "application/json")

			h.Register(w, r)

			if w.Code != tc.expectedStatus {
				t.Errorf("expected status %d, got %d", tc.expectedStatus, w.Code)
			}

			contentType := w.Header().Get("Content-Type")
			if contentType != "application/json" {
				t.Errorf("expected Content-Type application/json, got %s", contentType)
			}

			if tc.verifyBody != nil {
				tc.verifyBody(t, w.Body.Bytes())
			}
		})
	}
}
