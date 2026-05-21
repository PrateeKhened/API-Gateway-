// Package logger provides a structured JSON logger wrapped around Go's standard slog library.
// It supports context propagation of request IDs and custom log levels.
package logger

import (
	"context"
	"io"
	"log/slog"
	"os"
	"strings"
)

// contextKey is a private type to avoid context key collisions.
type contextKey string

const (
	loggerKey    contextKey = "logger"
	requestIDKey contextKey = "request_id"
)

// Logger wraps the slog.Logger to enforce project-specific requirements,
// such as injecting service name and request ID.
type Logger struct {
	inner     *slog.Logger
	service   string
	level     string
	requestID string
}

// NewLogger creates a new structured JSON logger for the specified service
// and log level. If an invalid log level is provided, it defaults to "info"
// and logs a warning.
func NewLogger(service string, level string) *Logger {
	return newLoggerWithWriter(os.Stdout, service, level)
}

// newLoggerWithWriter is an internal helper that initializes a logger with a custom io.Writer.
// This allows capturing output in tests.
func newLoggerWithWriter(w io.Writer, service string, level string) *Logger {
	var slogLevel slog.Level
	var invalidLevel bool

	switch strings.ToLower(level) {
	case "debug":
		slogLevel = slog.LevelDebug
	case "info":
		slogLevel = slog.LevelInfo
	case "warn":
		slogLevel = slog.LevelWarn
	case "error":
		slogLevel = slog.LevelError
	default:
		slogLevel = slog.LevelInfo
		invalidLevel = true
	}

	handler := slog.NewJSONHandler(w, &slog.HandlerOptions{
		Level: slogLevel,
	}).WithAttrs([]slog.Attr{slog.String("service", service)})

	l := &Logger{
		inner:   slog.New(handler),
		service: service,
		level:   strings.ToLower(level),
	}

	if invalidLevel {
		l.Warn("invalid log level, defaulting to info", "invalid_level", level)
	}

	return l
}

// NewNopLogger creates a no-op logger that discards all log outputs,
// suitable for testing.
func NewNopLogger() *Logger {
	handler := slog.NewJSONHandler(io.Discard, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	})
	return &Logger{
		inner:   slog.New(handler),
		service: "nop",
		level:   "debug",
	}
}

// WithRequestID returns a new context with the provided request ID attached.
// If a Logger is already present in the context, it binds the request ID to the logger
// and updates the Logger in the returned context to avoid downstream re-allocation.
func WithRequestID(ctx context.Context, requestID string) context.Context {
	ctx = context.WithValue(ctx, requestIDKey, requestID)
	if l, ok := ctx.Value(loggerKey).(*Logger); ok {
		if l.requestID != requestID {
			lBound := &Logger{
				inner:     l.inner.With(slog.String("request_id", requestID)),
				service:   l.service,
				level:     l.level,
				requestID: requestID,
			}
			ctx = context.WithValue(ctx, loggerKey, lBound)
		}
	}
	return ctx
}

// WithLogger returns a new context with the provided Logger attached.
// If a request ID is already present in the context, it binds it to the logger
// before storing it in the context to avoid downstream re-allocation.
func WithLogger(ctx context.Context, l *Logger) context.Context {
	if reqID, ok := ctx.Value(requestIDKey).(string); ok && reqID != "" {
		if l.requestID != reqID {
			l = &Logger{
				inner:     l.inner.With(slog.String("request_id", reqID)),
				service:   l.service,
				level:     l.level,
				requestID: reqID,
			}
		}
	}
	return context.WithValue(ctx, loggerKey, l)
}

// FromContext retrieves the Logger from context. If no logger is present,
// it returns a default logger. It automatically applies any request ID in the context.
func FromContext(ctx context.Context) *Logger {
	l, ok := ctx.Value(loggerKey).(*Logger)
	if !ok {
		// Create a fallback logger
		l = NewLogger("unknown", "info")
	}

	reqID, _ := ctx.Value(requestIDKey).(string)
	if reqID == "" {
		return l
	}

	// If the logger already has this request ID, return it to avoid re-allocation
	if l.requestID == reqID {
		return l
	}

	return &Logger{
		inner:     l.inner.With(slog.String("request_id", reqID)),
		service:   l.service,
		level:     l.level,
		requestID: reqID,
	}
}

// RequestIDFromContext retrieves the request ID from the context if it exists.
func RequestIDFromContext(ctx context.Context) string {
	if reqID, ok := ctx.Value(requestIDKey).(string); ok {
		return reqID
	}
	return ""
}

// Debug logs at the DEBUG level.
func (l *Logger) Debug(msg string, args ...any) {
	l.inner.Debug(msg, args...)
}

// Info logs at the INFO level.
func (l *Logger) Info(msg string, args ...any) {
	l.inner.Info(msg, args...)
}

// Warn logs at the WARN level.
func (l *Logger) Warn(msg string, args ...any) {
	l.inner.Warn(msg, args...)
}

// Error logs at the ERROR level.
func (l *Logger) Error(msg string, args ...any) {
	l.inner.Error(msg, args...)
}
