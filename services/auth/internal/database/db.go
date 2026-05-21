// Package database provides the PostgreSQL connection pool management
// and healthcheck utilities for the Auth service.
package database

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Config holds the configuration options for the database connection pool.
type Config struct {
	// DSN is the full database connection string (e.g. postgres://user:pass@host:port/db)
	DSN string
	// MaxConns is the maximum number of connections allowed in the pool.
	MaxConns int32
	// MinConns is the minimum number of idle connections maintained in the pool.
	MinConns int32
	// MaxConnLifetime is the maximum amount of time a connection may exist.
	MaxConnLifetime time.Duration
	// MaxConnIdleTime is the maximum amount of time an idle connection can remain.
	MaxConnIdleTime time.Duration
}

// New initializes and returns a configured pgxpool.Pool.
// It parses the DSN, applies the custom configuration overrides,
// and verifies database connectivity before returning.
func New(ctx context.Context, cfg Config) (*pgxpool.Pool, error) {
	config, err := pgxpool.ParseConfig(cfg.DSN)
	if err != nil {
		return nil, fmt.Errorf("parse dsn config: %w", err)
	}

	if cfg.MaxConns > 0 {
		config.MaxConns = cfg.MaxConns
	} else {
		config.MaxConns = 25
	}

	if cfg.MinConns > 0 {
		config.MinConns = cfg.MinConns
	} else {
		config.MinConns = 5
	}

	if cfg.MaxConnLifetime > 0 {
		config.MaxConnLifetime = cfg.MaxConnLifetime
	} else {
		config.MaxConnLifetime = 1 * time.Hour
	}

	if cfg.MaxConnIdleTime > 0 {
		config.MaxConnIdleTime = cfg.MaxConnIdleTime
	} else {
		config.MaxConnIdleTime = 30 * time.Minute
	}

	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("initialize database pool: %w", err)
	}

	conn, err := pool.Acquire(ctx)
	if err != nil {
		pool.Close()
		return nil, fmt.Errorf("acquire test connection: %w", err)
	}
	defer conn.Release()

	if err := conn.Conn().Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping test connection: %w", err)
	}

	return pool, nil
}

// Close cleanly shuts down the database connection pool if it is not nil.
func Close(pool *pgxpool.Pool) {
	if pool != nil {
		pool.Close()
	}
}

// HealthCheck verifies if the database connection pool is healthy.
// It acquires a connection from the pool and runs a minimal query.
func HealthCheck(ctx context.Context, pool *pgxpool.Pool) error {
	if pool == nil {
		return fmt.Errorf("database health check: pool is nil")
	}

	conn, err := pool.Acquire(ctx)
	if err != nil {
		return fmt.Errorf("acquire connection for health check: %w", err)
	}
	defer conn.Release()

	var one int
	err = conn.QueryRow(ctx, "SELECT 1").Scan(&one)
	if err != nil {
		return fmt.Errorf("execute health check query: %w", err)
	}

	return nil
}
