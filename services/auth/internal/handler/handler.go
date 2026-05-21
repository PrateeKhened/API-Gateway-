// Package handler implements the HTTP presentation layer for the Auth service.
package handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/yourusername/devplatform/pkg/apierror"
	"github.com/yourusername/devplatform/pkg/logger"
	"github.com/yourusername/devplatform/services/auth/db/generated"
	"github.com/yourusername/devplatform/services/auth/internal/service"
)

// Handler holds dependencies needed across all HTTP endpoints.
type Handler struct {
	svc interface {
		Register(ctx context.Context, email, password string) (dbgen.User, error)
	}
	logger *logger.Logger
}

// NewHandler constructs and returns a new Handler instance.
func NewHandler(svc *service.Service, log *logger.Logger) *Handler {
	return &Handler{
		svc:    svc,
		logger: log,
	}
}

// decodeJSON reads and parses JSON from the request body into the destination type.
// It enforces a 1MB limit on request size and returns true if parsing was successful,
// writing standard client-safe API errors directly on failure.
func (h *Handler) decodeJSON(w http.ResponseWriter, r *http.Request, dst any) bool {
	// Limit request body to 1MB
	r.Body = http.MaxBytesReader(w, r.Body, 1048576)

	dec := json.NewDecoder(r.Body)
	// Disallow unknown fields to catch typo bugs early
	dec.DisallowUnknownFields()

	if err := dec.Decode(dst); err != nil {
		var syntaxErr *json.SyntaxError
		var unmarshalTypeErr *json.UnmarshalTypeError
		var maxBytesErr *http.MaxBytesError

		switch {
		case errors.Is(err, io.EOF):
			apierror.Write(w, r, apierror.ErrBadRequest("request body is required"))
			return false

		case errors.Is(err, io.ErrUnexpectedEOF):
			apierror.Write(w, r, apierror.ErrBadRequest("malformed JSON body (unexpected end of input)"))
			return false

		case errors.As(err, &syntaxErr):
			msg := fmt.Sprintf("malformed JSON body (syntax error at byte %d)", syntaxErr.Offset)
			apierror.Write(w, r, apierror.ErrBadRequest(msg))
			return false

		case errors.As(err, &unmarshalTypeErr):
			msg := fmt.Sprintf("invalid value type for field %q (expected %s)", unmarshalTypeErr.Field, unmarshalTypeErr.Type.String())
			apierror.Write(w, r, apierror.ErrBadRequest(msg))
			return false

		case errors.As(err, &maxBytesErr):
			apierror.Write(w, r, apierror.ErrBadRequest("request body too large"))
			return false

		default:
			reqID := logger.RequestIDFromContext(r.Context())
			h.logger.Error("failed to decode json request body", "error", err, "request_id", reqID)
			apierror.Write(w, r, apierror.ErrInternal(reqID))
			return false
		}
	}

	// Verify request body contains exactly one JSON object
	if err := dec.Decode(&struct{}{}); err != nil && !errors.Is(err, io.EOF) {
		apierror.Write(w, r, apierror.ErrBadRequest("request body must contain a single JSON object"))
		return false
	}

	return true
}
