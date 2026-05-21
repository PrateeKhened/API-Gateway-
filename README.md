# API Gateway Platform Monorepo

A high-performance, decoupled API-as-a-Service infrastructure platform. This project provides the plumbing that allows other services/APIs to be exposed securely, authenticated, rate-limited, tracked, and billed in real-time.

## Key Architecture Concepts

* **Synchronous Critical Path:** A Go-based API Gateway that intercepts requests, checks authorization, and verifies rate limits against Redis before reverse-proxying to upstream servers. Done in under 25ms.
* **Asynchronous Background Path:** A decoupled telemetry pipeline. The Gateway streams completion logs into Redis Streams, which are consumed out-of-band by analytical and billing services.
* **Database Isolation:** Database-per-service architecture running isolated PostgreSQL database containers for the Auth and Usage services.

```
                          [ THE SYNCHRONOUS CRITICAL PATH ]
                                         │
Client ──► [ API GATEWAY ] ──────────────┼──────────────► [ CORE SERVICE ]
                │                        │               (e.g., Text Utility)
                ├──► Auth Check (JWT)    │
                ├──► Rate Limit (Redis)  │
                └──► Cache Check (Redis) │
                                         │
                    ⚡ ASYNC EVENT LINE  ▼  (Fire & Forget)
          ─────────────────────────────────────────────────────────────
                          [ THE ASYNC BACKGROUND PATH ]
                                         │
                                  [ REDIS STREAMS ]
                                         │
                         ┌───────────────┴───────────────┐
                         ▼                               ▼
               [ USAGE SERVICE ]                [ BILLING SERVICE ]
             (Increments Quotas)               (Processes Payments)
```

For a detailed walkthrough of the request lifecycle, see [gemini.md](./gemini.md).

---

## Directory Structure

```
/api-platform-monorepo
├── services/                    # Independently deployable microservices
│   ├── auth/                    # Auth service
│   │   ├── cmd/                 # Service entry point (main.go)
│   │   ├── migrations/          # Auth database schema migrations (000001_...)
│   │   ├── db/                  # sqlc config, queries, and generated files
│   │   │   ├── sqlc.yaml        # sqlc configuration
│   │   │   ├── queries/         # raw SQL query definitions
│   │   │   └── generated/       # type-safe Go generated code (dbgen)
│   │   └── internal/            # Private auth-specific application code
│   │       ├── database/        # pgx database connection pool management
│   │       ├── handler/         # HTTP presentations & request decoders (Register)
│   │       └── service/         # Domain workflow & business validations
│   ├── gateway/                 # API Gateway service
│   ├── usage/                   # Usage tracking service
│   └── billing/                 # Billing service
├── pkg/                         # Shared utilities across services
│   ├── logger/                  # Structured JSON logger with request ID support
│   └── apierror/                # Shared HTTP API error handler
├── docker-compose.yml           # Local multi-container orchestration
└── README.md                    # System architecture & documentation
```

---

## Running the Auth Service

To run the Auth Service locally:
1. Ensure your local docker environment is running.
2. Run the application passing the DSN:
```bash
export DB_DSN="postgres://platform_user:platform_secure_password@localhost:5431/auth_db?sslmode=disable"
cd services/auth
go run cmd/main.go
```


---

## Getting Started

### Prerequisites

To build and run this project, you will need:
* **Go:** `v1.22` or later
* **Docker & Docker Compose**
* **sqlc:** For database code generation (`go install github.com/sqlc-dev/sqlc/cmd/sqlc@latest`)

### Setup Local Infrastructure

Spin up the isolated PostgreSQL databases and Redis instances:
```bash
docker compose up -d
```

Verify that all services are running:
```bash
docker compose ps
```

### Database Migrations & SQLC

When adding migrations to a service (e.g. `services/auth/migrations/`), run the `sqlc` tool to regenerate Go models and queries:
```bash
sqlc generate
```

---

## Roadmap

1. **Milestone 1: The Identity Backbone (Auth Service)** - User registration, bcrypt password hashing, JWT session management, refresh tokens, and API key generation (`sk_live_...`).
2. **Milestone 2: The Traffic Controller (API Gateway)** - Route configuration parser, reverse proxy, and Redis-backed rate limiting.
3. **Milestone 3: The Async Bookkeeper (Usage Tracking Service)** - Redis Streams integration, event consumers, and batch PostgreSQL analytics writes.
4. **Milestone 4: The Financial Engine (Billing Service)** - Idempotent billing invoice processing and plan tier control.
