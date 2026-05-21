// Package apierror provides a standardized structure for handling and returning
// API errors consistently across all microservices in the platform.
package apierror

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/yourusername/devplatform/pkg/logger"
)

// APIError represents a structured error payload returned to API consumers.
// It supports details for validation errors and carries HTTP status code internally.
type APIError struct {
	Code       int      `json:"-"`
	Message    string   `json:"message"`
	ErrorCode  string   `json:"code"`
	RequestID  string   `json:"request_id"`
	Details    []string `json:"details,omitempty"`
	retryAfter int
}

// Error implements the standard Go error interface.
func (e *APIError) Error() string {
	if len(e.Details) > 0 {
		return fmt.Sprintf("[%s] %s: %v", e.ErrorCode, e.Message, e.Details)
	}
	return fmt.Sprintf("[%s] %s", e.ErrorCode, e.Message)
}

// ErrBadRequest constructs a 400 Bad Request error.
func ErrBadRequest(message string, details ...string) *APIError {
	var d []string
	if len(details) > 0 {
		d = details
	}
	return &APIError{
		Code:      http.StatusBadRequest,
		Message:   message,
		ErrorCode: "INVALID_INPUT",
		Details:   d,
	}
}

// ErrUnauthorized constructs a 401 Unauthorized error.
func ErrUnauthorized(message string) *APIError {
	return &APIError{
		Code:      http.StatusUnauthorized,
		Message:   message,
		ErrorCode: "UNAUTHORIZED",
	}
}

// ErrForbidden constructs a 403 Forbidden error.
func ErrForbidden(message string) *APIError {
	return &APIError{
		Code:      http.StatusForbidden,
		Message:   message,
		ErrorCode: "FORBIDDEN",
	}
}

// ErrNotFound constructs a 404 Not Found error and includes the resource name in the message.
func ErrNotFound(resource string) *APIError {
	return &APIError{
		Code:      http.StatusNotFound,
		Message:   fmt.Sprintf("%s not found", resource),
		ErrorCode: "NOT_FOUND",
	}
}

// ErrConflict constructs a 409 Conflict error with a custom machine-readable code.
func ErrConflict(message string, errorCode string) *APIError {
	return &APIError{
		Code:      http.StatusConflict,
		Message:   message,
		ErrorCode: errorCode,
	}
}

// ErrUnprocessable constructs a 422 Unprocessable Entity error.
func ErrUnprocessable(message string, details ...string) *APIError {
	var d []string
	if len(details) > 0 {
		d = details
	}
	return &APIError{
		Code:      http.StatusUnprocessableEntity,
		Message:   message,
		ErrorCode: "UNPROCESSABLE_ENTITY",
		Details:   d,
	}
}

// ErrTooManyRequests constructs a 429 Too Many Requests error with rate limiting headers setup.
func ErrTooManyRequests(retryAfter int) *APIError {
	return &APIError{
		Code:       http.StatusTooManyRequests,
		Message:    "Rate limit exceeded",
		ErrorCode:  "RATE_LIMITED",
		retryAfter: retryAfter,
	}
}

// ErrInternal constructs a 500 Internal Server Error. It suppresses the original error details to avoid leak.
func ErrInternal(requestID string) *APIError {
	return &APIError{
		Code:      http.StatusInternalServerError,
		Message:   "Internal server error",
		ErrorCode: "INTERNAL_ERROR",
		RequestID: requestID,
	}
}

type responseEnvelope struct {
	Error *APIError `json:"error"`
}

// Write sets headers and writes the APIError to the http.ResponseWriter.
// It retrieves the request ID from context to synchronize with the error.
func Write(w http.ResponseWriter, r *http.Request, err *APIError) {
	if err == nil {
		err = ErrInternal("")
	}

	// Try to get Request ID from the context if it is set and not already present in the error
	if reqID := logger.RequestIDFromContext(r.Context()); reqID != "" {
		err.RequestID = reqID
	}

	w.Header().Set("Content-Type", "application/json")

	if err.Code == http.StatusTooManyRequests && err.retryAfter > 0 {
		w.Header().Set("Retry-After", strconv.Itoa(err.retryAfter))
	}

	envelope := responseEnvelope{
		Error: err,
	}

	// Marshal response in-memory first to avoid partial writes on serialization error
	res, marshalErr := json.Marshal(envelope)
	if marshalErr != nil {
		w.WriteHeader(http.StatusInternalServerError)
		// Safe fallback string representation
		_, _ = w.Write([]byte(`{"error":{"code":"INTERNAL_ERROR","message":"Internal server error"}}`))
		return
	}

	w.WriteHeader(err.Code)
	_, _ = w.Write(res)
}
