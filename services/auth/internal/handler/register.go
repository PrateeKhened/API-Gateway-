package handler

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/yourusername/devplatform/pkg/apierror"
	"github.com/yourusername/devplatform/pkg/logger"
	"github.com/yourusername/devplatform/services/auth/internal/service"
)

// RegisterRequest defines the input payload for the user registration endpoint.
type RegisterRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// RegisterResponse defines the successful output envelope of registration.
type RegisterResponse struct {
	User UserResponse `json:"user"`
}

// UserResponse defines the client-safe public user fields.
type UserResponse struct {
	ID         string `json:"id"`
	Email      string `json:"email"`
	IsVerified bool   `json:"is_verified"`
	CreatedAt  string `json:"created_at"`
}

// Register handles HTTP requests to register a new user account.
func (h *Handler) Register(w http.ResponseWriter, r *http.Request) {
	var req RegisterRequest
	if !h.decodeJSON(w, r, &req) {
		return
	}

	user, err := h.svc.Register(r.Context(), req.Email, req.Password)
	if err != nil {
		reqID := logger.RequestIDFromContext(r.Context())

		if errors.Is(err, service.ErrEmailTaken) {
			apierror.Write(w, r, apierror.ErrConflict("email already registered", "EMAIL_ALREADY_EXISTS"))
			return
		}

		var valErr *service.ValidationError
		if errors.As(err, &valErr) {
			apierror.Write(w, r, apierror.ErrUnprocessable("validation failed", valErr.Fields...))
			return
		}

		h.logger.Error("failed to register user in service layer", "error", err, "request_id", reqID)
		apierror.Write(w, r, apierror.ErrInternal(reqID))
		return
	}

	createdAtStr := ""
	if user.CreatedAt.Valid {
		createdAtStr = user.CreatedAt.Time.UTC().Format(time.RFC3339)
	}

	resp := RegisterResponse{
		User: UserResponse{
			ID:         formatUUID(user.ID.Bytes),
			Email:      user.Email,
			IsVerified: user.IsVerified,
			CreatedAt:  createdAtStr,
		},
	}

	if err := writeJSON(w, http.StatusCreated, resp); err != nil {
		reqID := logger.RequestIDFromContext(r.Context())
		h.logger.Error("failed to write register response json", "error", err, "request_id", reqID)
	}
}

// writeJSON formats and writes the payload as JSON to w.
func writeJSON(w http.ResponseWriter, status int, data any) error {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		return fmt.Errorf("write json: %w", err)
	}
	return nil
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
