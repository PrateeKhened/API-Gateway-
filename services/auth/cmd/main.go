// Package main implements the entrypoint for the Auth service.
package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/yourusername/devplatform/pkg/logger"
	"github.com/yourusername/devplatform/services/auth/internal/database"
	"github.com/yourusername/devplatform/services/auth/internal/handler"
	"github.com/yourusername/devplatform/services/auth/internal/service"
)

func main() {
	// 1. Read config from environment variables
	dbDSN := os.Getenv("DB_DSN")
	logLevel := os.Getenv("LOG_LEVEL")
	if logLevel == "" {
		logLevel = "info"
	}
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	serviceName := os.Getenv("SERVICE_NAME")
	if serviceName == "" {
		serviceName = "auth"
	}

	// 2. Validate DB_DSN before initializing anything else
	if dbDSN == "" {
		fmt.Fprintln(os.Stderr, "Error: DB_DSN environment variable is required")
		os.Exit(1)
	}

	// 3. Initialize in strict order
	// a. Logger
	log := logger.NewLogger(serviceName, logLevel)

	// b. Database pool
	dbCtx, dbCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer dbCancel()
	dbPool, err := database.New(dbCtx, database.Config{
		DSN: dbDSN,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error initializing database pool: %v\n", err)
		os.Exit(1)
	}
	defer database.Close(dbPool)

	// c. Service
	svc := service.NewService(dbPool, log)

	// d. Handler
	h := handler.NewHandler(svc, log)

	// e. Router
	r := chi.NewRouter()

	// Apply standard middlewares
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)

	// Custom logging middleware that binds request ID to logger context
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()

			reqID := middleware.GetReqID(r.Context())
			if reqID == "" {
				reqID = "unknown"
			}

			// Add request ID and logger to context
			ctx := logger.WithRequestID(r.Context(), reqID)
			ctx = logger.WithLogger(ctx, log)

			ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)

			next.ServeHTTP(ww, r.WithContext(ctx))

			reqLogger := logger.FromContext(ctx)
			reqLogger.Info("request completed",
				"method", r.Method,
				"path", r.URL.Path,
				"status", ww.Status(),
				"bytes", ww.BytesWritten(),
				"latency_ms", time.Since(start).Milliseconds(),
			)
		})
	})

	// Mount the register route
	r.Post("/auth/register", h.Register)

	// f. HTTP server
	addr := ":" + port
	server := &http.Server{
		Addr:         addr,
		Handler:      r,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	// 6. Graceful shutdown
	serverErrors := make(chan error, 1)

	go func() {
		log.Info("server starting", "addr", addr)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErrors <- err
		}
	}()

	shutdownSig := make(chan os.Signal, 1)
	signal.Notify(shutdownSig, os.Interrupt, syscall.SIGTERM)

	select {
	case err := <-serverErrors:
		fmt.Fprintf(os.Stderr, "Error starting server: %v\n", err)
		os.Exit(1)

	case sig := <-shutdownSig:
		log.Info("server shutting down", "signal", sig.String())

		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer shutdownCancel()

		if err := server.Shutdown(shutdownCtx); err != nil {
			log.Error("graceful shutdown failed, forcing close", "error", err)
			_ = server.Close()
		}

		log.Info("server stopped")
	}
}
