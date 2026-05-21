# Architectural Reference: Gemini Conversation & Roadmap

This document serves as the foundational design blueprint and roadmap for the API Gateway and Infrastructure project. It details the system architecture, request flows, resume positioning strategy, and incremental project milestones.

---

## 1. What is this Project?

This project builds the core infrastructure of an **API-as-a-Service Platform** (similar to a mini-Stripe, RapidAPI, or OpenAI). It provides the essential plumbing to expose APIs safely, track consumption, and manage quotas and billing.

The interesting part of the project is not the upstream API services themselves (which can be simple utilities like text analysis or geolocation), but rather the **infrastructure wrapping around them**:

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

---

## 2. Why this System matters for Engineering Depth

Instead of standard CRUD apps, this project focuses on real computer science and distributed system trade-offs:
* **Concurrency & Performance:** Implementing a Go-based gateway to handle HTTP proxying with minimal overhead, utilizing Goroutines and standard library efficiency.
* **Decoupled Architecture:** Using **Redis Streams** as an event broker between the Gateway and downstream bookkeeping services (Usage and Billing). The critical path remains active even if PostgreSQL databases go down.
* **Database Isolation (Database per Service):** Each microservice owns its database schemas and PostgreSQL instance, preventing tight coupling and cascading database failures.
  * `auth_db` stores users, api_keys, and sessions.
  * `billing_db` manages plans, invoices, and events.
  * `usage_db` tracks api_calls, daily_rollups, and quotas.
* **No "Magic" Libraries:** Using **sqlc** instead of heavy ORMs to generate type-safe Go code directly from raw, optimized SQL queries.

---

## 3. The Master Strategy: Incremental Milestones

An **Incremental Delivery Model** is used to build the system piece-by-piece, ensuring a functional, demoable, and shippable asset at each stage.

### Milestone 1: The Identity Backbone (Auth Service)
* **What we build:** User registration, password hashing (bcrypt), session management with JWT access tokens, long-lived refresh token rotation, API key generation (e.g., prefix `sk_live_...`), and security extensions like TOTP 2FA and OAuth2.
* **Why now:** Establishes core database schemas, initializes migrations, and isolates identity/auth domains early.
* **Progress:**
  * **[x] Chunk 1.1 — Schema & Migrations**: Designed relational schemas for tables `users`, `refresh_tokens`, and `api_keys` with cascading deletes and index optimizations.
  * **[x] Chunk 1.2 — SQLC & DB Connection Pool**: Setup sqlc Go generator, queries, and constructed a robust database pool manager with verify-on-startup and health check hooks.
  * **[x] Chunk 1.3 — User Registration Handler**: Implemented `POST /auth/register` endpoint using standard library structured logging, password complexity validation rules, bcrypt hashing, and graceful shutdown orchestration. Tested via table-driven testing and manual execution scripts.

### Milestone 2: The Traffic Controller (API Gateway)
* **What we build:** A reverse proxy built using Go's `net/http/httputil`, a dynamic configuration engine reading routes from a YAML file, a token-bucket rate limiter backed by Redis, and a caching layer (TTL lookup).
* **Why now:** Officially introduces a distributed system. Allows a demo of a request passing through the gateway, being validated, and successfully proxying to an upstream service.

### Milestone 3: The Async Bookkeeper (Usage Tracking Service)
* **What we build:** The Gateway fires a background event (`request.done`) to Redis Streams on request completion. The Usage Service consumes this via a consumer group to record every API call, batches the database writes to PostgreSQL to avoid lock contention, generates daily & monthly analytics rollups, and emits a `quota.exceeded` event if limits are breached.
* **Why now:** Implements the event-driven architecture, illustrating decoupled microservices handling high-throughput analytical writes.

### Milestone 4: The Financial Engine (Billing Service)
* **What we build:** Plan/subscription management, invoice generation, dunning and payment retry flows, and idempotent invoice processing using idempotency keys to ensure safety across retries.

---

## 4. Production-Grade Monorepo Layout

We organize the services in a single repository while strictly isolating their domains and codebases:

```
/api-platform-monorepo
├── cmd/                         # Service Entry Points
│   ├── auth/                    # Auth service binary main.go
│   └── gateway/                 # Gateway service binary main.go
├── services/                    # Independently deployable microservices
│   ├── auth/                    # Auth service
│   │   ├── migrations/          # Auth database schema migrations (000001_...)
│   │   ├── db/                  # sqlc config, queries, and generated files
│   │   │   ├── sqlc.yaml        # sqlc configuration
│   │   │   ├── queries/         # raw SQL query definitions
│   │   │   └── generated/       # type-safe Go generated code (dbgen)
│   │   └── internal/            # Private auth-specific application code
│   │       └── database/        # pgx database connection pool management
│   ├── gateway/                 # API Gateway service
│   ├── usage/                   # Usage tracking service
│   │   └── migrations/          # Usage database schema migrations
│   └── billing/                 # Billing service
│       └── migrations/          # Billing database schema migrations
├── pkg/                         # Shared utilities across services
│   ├── logger/                  # Structured JSON logger with request ID support
│   ├── apierror/                # Shared HTTP API error handler
│   └── tokens/                  # Common JWT parsing/claims utilities
├── docker-compose.yml           # Local multi-container orchestration
├── sqlc.yaml                    # Code generator configuration (global template)
├── go.mod                       # Global dependencies list
└── README.md                    # System architecture & documentation
```

---

## 5. Detailed Request Lifecycle

Here is a step-by-step trace of what happens when a client app queries the platform. 

### Part 1: The Synchronous Critical Path (The Fast Lane)

Imagine a developer client calls the platform:
`POST http://api.yourplatform.com/v1/text/summarize` with `Authorization: Bearer sk_live_abc123`.

```
Rahul's Client App
       │
       │ (1) HTTP POST /v1/text/summarize
       ▼
┌────────────────────────────────────────────────────────────────────────┐
│                          1. THE API GATEWAY                            │
│                                                                        │
│  [Auth Middleware] ──(2) Check Local Cache/Memory? ────────────────┐  │
│         │                                                          │  │
│         ▼ (3) Cache Miss -> Call Auth Service via HTTP REST        │  │
│   [Auth Service] ──(4) Query DB -> Verified: User ID 99            │  │
│         │                                                          │  │
│         ▼ (5) Token Valid!                                         │  │
│  [Rate Limiter] ───(6) Check Redis (Token Bucket) -> 43/1000 -> OK │  │
│         │                                                          │  │
│         ▼ (7) Route Request                                        │  │
│  [Reverse Proxy] ──(8) Forward HTTP request to Upstream Service    │  │
└─────────┬──────────────────────────────────────────────────────────┘
          │
          ▼ (9) Process Text & Return 200 OK
┌────────────────────────────────────────────────────────────────────────┐
│                        2. UPSTREAM SERVICE                             │
│                      (Text Utility Endpoint)                           │
└─────────┬──────────────────────────────────────────────────────────┘
          │
          ▼ (10) Capture response, forward back to Client
┌────────────────────────────────────────────────────────────────────────┐
│                         3. BACK TO CLIENT                              │
│         Rahul gets his HTTP 200 OK Response instantly!                 │
└────────────────────────────────────────────────────────────────────────┘
```

1. **Gateway Entry:** The API Gateway intercepts the request at port `:8080`.
2. **Auth Verification:**
   * The Gateway extracts `sk_live_abc123` and checks its local memory or Redis cache.
   * If there is a cache miss, the Gateway calls the Auth Service synchronously via `GET /internal/validate-key?key=sk_live_abc123`.
   * The Auth Service runs a SHA-256 hash on the token, validates it against PostgreSQL, and returns `200 OK` with JSON: `{ "user_id": 99, "plan": "free" }`.
   * The Gateway caches this result and injects the header `X-User-ID: 99` into the proxy context. (Invalid keys get dropped immediately with `401 Unauthorized`).
3. **Rate Limiting:**
   * The Gateway issues an atomic Redis script check: `EVAL token_bucket_script 2 rate_limit:99 10 1`.
   * Redis decrements the bucket. If tokens remain (e.g., 43/1000), it returns OK. If exhausted, the Gateway aborts with `429 Too Many Requests`.
4. **Reverse Proxying:**
   * The Gateway reads its `routes.yaml` file, matches `/v1/text/*` to `http://localhost:8081` (upstream text service), and uses Go's `httputil.NewSingleHostReverseProxy` to forward the request.
5. **Upstream Response:**
   * The Text Service processes the text, returns `200 OK` with summary JSON.
   * The Gateway streams this response back to the client and closes the connection. Time elapsed: **~25ms**.

---

### Part 2: The Asynchronous Background Path (The Bookkeeping)

Once the connection to the client is closed, background processing begins.

```
        Gateway Closes Connection to Rahul
                       │
                       ▼
       (1) Fire-and-Forget: Publish Message
             ┌───────────────────┐
             │   REDIS STREAMS   │  <- [Topic: request.done]
             └─────────┬─────────┘
                       │
         ┌─────────────┴─────────────┐
         ▼ (Async Consumer Group)    ▼ (Async Consumer Group)
┌──────────────────┐       ┌──────────────────┐
│  USAGE SERVICE   │       │  BILLING SERVICE │
│                  │       │                  │
│ (2) Batch Append │       │ (4) Check        │
│     to Postgres  │       │     Idempotency  │
│                  │       │                  │
│ (3) Check Quota  │       │ (5) Update       │
│     Hit 100%?    │       │     Ledger       │
└────────┬─────────┘       └──────────────────┘
         │
         ▼ Yes! Publish: [Topic: quota.exceeded]
┌──────────────────┐
│   REDIS STREAM   │
└────────┬─────────┘
         │
         ▼ (Consumer)
┌──────────────────┐
│   API GATEWAY    │ -> Blocks User 99 immediately on next call
└──────────────────┘
```

1. **Publish Event:** The Gateway fires a structured log message into Redis Streams:
   ```go
   redis.XAdd("request.done", map[string]interface{}{
       "user_id":   99,
       "endpoint":  "/v1/text/summarize",
       "status":    200,
       "timestamp": 1714210000,
   })
   ```
2. **Asynchronous Ingestion (Usage Service):**
   * The Usage Service consumes events from the `request.done` stream.
   * To prevent database performance degradation, it batches events in memory for 1 second or until it collects 100 entries, then performs a bulk SQL insert into PostgreSQL.
   * It increments monthly usage rollups in PostgreSQL (e.g., User 99 = 1,001 calls).
3. **Quota Violations:**
   * If the monthly quota is breached, the Usage Service publishes a `quota.exceeded` event back to Redis Streams.
   * The API Gateway (and other consumer instances) consume this event and instantly flag `block:99 = true` in Redis or local memory. Future requests from User 99 are rejected at the Gateway gateway before hitting upstream services or PostgreSQL.
