package logger

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"
)

// Helper to parse JSON log line into a map.
func parseLogLine(t *testing.T, line []byte) map[string]any {
	t.Helper()
	var result map[string]any
	if err := json.Unmarshal(line, &result); err != nil {
		t.Fatalf("failed to unmarshal JSON log: %v (input: %s)", err, string(line))
	}
	return result
}

// TestLogger_JSONOutputFields tests that JSON output contains all required fields (time, level, msg, service).
func TestLogger_JSONOutputFields(t *testing.T) {
	var buf bytes.Buffer
	l := newLoggerWithWriter(&buf, "auth", "info")
	l.Info("test message")

	output := buf.Bytes()
	logMap := parseLogLine(t, output)

	// Validate required fields
	required := []string{"time", "level", "msg", "service"}
	for _, field := range required {
		val, ok := logMap[field]
		if !ok {
			t.Errorf("missing required field: %q", field)
		}
		if str, isStr := val.(string); isStr && str == "" {
			t.Errorf("field %q is empty", field)
		}
	}

	if logMap["service"] != "auth" {
		t.Errorf("expected service to be 'auth', got %v", logMap["service"])
	}
	if logMap["level"] != "INFO" {
		t.Errorf("expected level to be 'INFO', got %v", logMap["level"])
	}
	if logMap["msg"] != "test message" {
		t.Errorf("expected msg to be 'test message', got %v", logMap["msg"])
	}
}

// TestLogger_RequestIDInContext tests that request_id appears in output when context carries one.
func TestLogger_RequestIDInContext(t *testing.T) {
	var buf bytes.Buffer
	l := newLoggerWithWriter(&buf, "auth", "info")

	ctx := context.Background()
	ctx = WithLogger(ctx, l)
	ctx = WithRequestID(ctx, "req_12345")

	loggerFromCtx := FromContext(ctx)
	loggerFromCtx.Info("test message with request ID")

	logMap := parseLogLine(t, buf.Bytes())
	reqID, ok := logMap["request_id"]
	if !ok {
		t.Fatalf("expected 'request_id' field in log output")
	}
	if reqID != "req_12345" {
		t.Errorf("expected request_id to be 'req_12345', got %v", reqID)
	}
}

// TestLogger_NoRequestIDInContext tests that request_id is absent from output when context has none.
func TestLogger_NoRequestIDInContext(t *testing.T) {
	var buf bytes.Buffer
	l := newLoggerWithWriter(&buf, "auth", "info")

	ctx := context.Background()
	ctx = WithLogger(ctx, l)

	loggerFromCtx := FromContext(ctx)
	loggerFromCtx.Info("test message without request ID")

	logMap := parseLogLine(t, buf.Bytes())
	if _, ok := logMap["request_id"]; ok {
		t.Errorf("expected no 'request_id' in log output, but it was present")
	}
}

// TestLogger_NewNopLoggerDiscards tests that NewNopLogger discards all output.
func TestLogger_NewNopLoggerDiscards(t *testing.T) {
	l := NewNopLogger()
	l.Debug("debug msg")
	l.Info("info msg")
	l.Warn("warn msg")
	l.Error("error msg")
}

// TestLogger_InvalidLogLevelDefaultsToInfo tests that invalid log level defaults to info.
func TestLogger_InvalidLogLevelDefaultsToInfo(t *testing.T) {
	var buf bytes.Buffer
	l := newLoggerWithWriter(&buf, "auth", "invalid-level")

	// The initialization warning will be printed to buf because it's an invalid log level,
	// but the debug level log should be silenced since it defaulted to info.
	l.Debug("debug message")
	if strings.Contains(buf.String(), "debug message") {
		t.Errorf("debug message was logged but level should default to info")
	}

	buf.Reset()
	l.Info("info message")
	if buf.Len() == 0 {
		t.Errorf("info message should be logged")
	}
	logMap := parseLogLine(t, buf.Bytes())
	if logMap["level"] != "INFO" {
		t.Errorf("expected level to default to INFO, got %v", logMap["level"])
	}
}

// TestLogger_DebugSuppressedAtInfo tests that debug messages are suppressed when level is "info".
func TestLogger_DebugSuppressedAtInfo(t *testing.T) {
	var buf bytes.Buffer
	l := newLoggerWithWriter(&buf, "auth", "info")
	l.Debug("debug message")
	if buf.Len() > 0 {
		t.Errorf("debug log should be suppressed at info level, got: %s", buf.String())
	}
}

// TestLogger_FromContextNoReallocation tests that logger from FromContext carries request ID without re-initialization.
func TestLogger_FromContextNoReallocation(t *testing.T) {
	var buf bytes.Buffer
	l := newLoggerWithWriter(&buf, "auth", "info")

	ctx := context.Background()
	ctx = WithLogger(ctx, l)
	ctx = WithRequestID(ctx, "req_abc")

	logger1 := FromContext(ctx)
	logger2 := FromContext(ctx)

	// Since request ID was pre-bound in context updates, the logger from FromContext
	// should return the exact same pre-allocated instance.
	if logger1 != logger2 {
		t.Errorf("expected same logger instance, but got different pointers: %p vs %p", logger1, logger2)
	}
}
