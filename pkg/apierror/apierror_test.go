package apierror

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/yourusername/devplatform/pkg/logger"
)

// Helper to parse response body into a map.
func parseResponse(t *testing.T, body []byte) map[string]any {
	t.Helper()
	var result map[string]any
	if err := json.Unmarshal(body, &result); err != nil {
		t.Fatalf("failed to unmarshal JSON response: %v", err)
	}
	return result
}

// TestAPIError_ConstructorStatusCodes checks that each constructor produces the correct HTTP status code.
func TestAPIError_ConstructorStatusCodes(t *testing.T) {
	tests := []struct {
		name         string
		err          *APIError
		expectedCode int
	}{
		{"BadRequest", ErrBadRequest("bad request"), http.StatusBadRequest},
		{"Unauthorized", ErrUnauthorized("unauthorized"), http.StatusUnauthorized},
		{"Forbidden", ErrForbidden("forbidden"), http.StatusForbidden},
		{"NotFound", ErrNotFound("User"), http.StatusNotFound},
		{"Conflict", ErrConflict("conflict", "EMAIL_EXISTS"), http.StatusConflict},
		{"Unprocessable", ErrUnprocessable("unprocessable"), http.StatusUnprocessableEntity},
		{"TooManyRequests", ErrTooManyRequests(60), http.StatusTooManyRequests},
		{"Internal", ErrInternal("req_123"), http.StatusInternalServerError},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.err.Code != tt.expectedCode {
				t.Errorf("expected HTTP status code %d, got %d", tt.expectedCode, tt.err.Code)
			}
		})
	}
}

// TestAPIError_InternalNeverExposesMessage checks that ErrInternal response body never contains original error details.
func TestAPIError_InternalNeverExposesMessage(t *testing.T) {
	err := ErrInternal("req_abc")
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/test", nil)

	Write(w, r, err)

	respMap := parseResponse(t, w.Body.Bytes())
	errMap, ok := respMap["error"].(map[string]any)
	if !ok {
		t.Fatalf("expected nested 'error' object in response")
	}

	msg, ok := errMap["message"].(string)
	if !ok {
		t.Fatalf("missing message field")
	}
	if msg != "Internal server error" {
		t.Errorf("expected message to be 'Internal server error', got %q", msg)
	}
}

// TestAPIError_TooManyRequestsRetryAfter checks that ErrTooManyRequests sets Retry-After header.
func TestAPIError_TooManyRequestsRetryAfter(t *testing.T) {
	err := ErrTooManyRequests(42)
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/test", nil)

	Write(w, r, err)

	if w.Code != http.StatusTooManyRequests {
		t.Errorf("expected status code %d, got %d", http.StatusTooManyRequests, w.Code)
	}

	retryAfterHeader := w.Header().Get("Retry-After")
	if retryAfterHeader != "42" {
		t.Errorf("expected Retry-After header to be '42', got %q", retryAfterHeader)
	}
}

// TestAPIError_JSONShape matches the required structure exactly.
func TestAPIError_JSONShape(t *testing.T) {
	err := ErrBadRequest("Invalid inputs", "email is invalid", "password too short")
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/test", nil)

	// Inject a request ID into the context to test propagation
	ctx := logger.WithRequestID(r.Context(), "req_uuid_999")
	r = r.WithContext(ctx)

	Write(w, r, err)

	// Validate status code and content type
	if w.Header().Get("Content-Type") != "application/json" {
		t.Errorf("expected Content-Type application/json, got %q", w.Header().Get("Content-Type"))
	}

	respMap := parseResponse(t, w.Body.Bytes())
	errMap, ok := respMap["error"].(map[string]any)
	if !ok {
		t.Fatalf("expected 'error' field containing error payload")
	}

	expectedKeys := map[string]string{
		"code":       "INVALID_INPUT",
		"message":    "Invalid inputs",
		"request_id": "req_uuid_999",
	}

	for k, expectedVal := range expectedKeys {
		val, ok := errMap[k]
		if !ok {
			t.Errorf("missing key %q in error payload", k)
		}
		if val != expectedVal {
			t.Errorf("key %q: expected %q, got %q", k, expectedVal, val)
		}
	}

	detailsVal, ok := errMap["details"]
	if !ok {
		t.Fatalf("missing details field in error payload")
	}

	detailsSlice, ok := detailsVal.([]any)
	if !ok {
		t.Fatalf("details should be a JSON array")
	}

	if len(detailsSlice) != 2 || detailsSlice[0] != "email is invalid" || detailsSlice[1] != "password too short" {
		t.Errorf("unexpected details contents: %v", detailsSlice)
	}
}

// TestAPIError_WriteWithCancelledContext checks that Write with a cancelled context still writes a valid response.
func TestAPIError_WriteWithCancelledContext(t *testing.T) {
	err := ErrForbidden("access denied")
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/test", nil)

	// Cancel context
	ctx, cancel := context.WithCancel(r.Context())
	cancel()
	r = r.WithContext(ctx)

	Write(w, r, err)

	if w.Code != http.StatusForbidden {
		t.Errorf("expected HTTP status code %d even under cancelled context, got %d", http.StatusForbidden, w.Code)
	}

	respMap := parseResponse(t, w.Body.Bytes())
	errMap, ok := respMap["error"].(map[string]any)
	if !ok {
		t.Fatalf("expected error object in response")
	}
	if errMap["code"] != "FORBIDDEN" {
		t.Errorf("expected error code to be FORBIDDEN, got %v", errMap["code"])
	}
}

// TestAPIError_DetailsOmittedWhenEmpty checks that Details field is omitted from JSON when empty (omitempty working).
func TestAPIError_DetailsOmittedWhenEmpty(t *testing.T) {
	err := ErrBadRequest("Single error message") // details is empty
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/test", nil)

	Write(w, r, err)

	respMap := parseResponse(t, w.Body.Bytes())
	errMap, ok := respMap["error"].(map[string]any)
	if !ok {
		t.Fatalf("expected error object in response")
	}

	if _, ok := errMap["details"]; ok {
		t.Errorf("expected 'details' field to be omitted from JSON output, but it was present")
	}
}

// TestAPIError_NotFoundContainsResource checks that ErrNotFound includes the resource name in the message.
func TestAPIError_NotFoundContainsResource(t *testing.T) {
	err := ErrNotFound("BillingPlan")
	if !strings.Contains(err.Message, "BillingPlan") {
		t.Errorf("expected message to contain 'BillingPlan', got %q", err.Message)
	}
}
