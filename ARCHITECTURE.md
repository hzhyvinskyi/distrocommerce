# DistroCommerce — Architecture

## Overview

DistroCommerce is a production-ready distributed e-commerce platform built with Go. It consists of multiple microservices that communicate via gRPC and Kafka, and deployed on Kubernetes.

```
Browser
  └── Nginx Ingress <- TLS termination, L7 routing by subdomain/path
      └── API Gateway (:8080) <- HTTP proxy, JWKS auth, rate limiter, circuit breaker
          └── BFF Web (:8090) <- OAuth2 confidential client, session store, grpc-gateway
              ├── user-sv      (:50051) <- profile CRUD
              ├── product-sv   (:50052) <- catalog, MongoDB
              ├── order-sv     (:50053) <- saga orchestrator, transactional outbox
              ├── payment-sv   (:50054) <- Stripe, idempotency
              ├── inventory-sv (:50055) <- pessimistic locking
              └── identity-sv  (:8087)  <- OAuth2 Authorization Server, HTTP-only

Identity Service (:8087) <- OAuth2 Authorization Server, HTTP-only
  ├── BFF Web calls directly: POST /oauth2/token, POST /oauth2/logout
  └── API Gateway validates JWT locally via cached JWKS — zero identity calls per request

Kafka (async)
  identity -> user-sv      (identity.registered)
  order    -> notification (order.confirmed, order.cancelled)
  payment  -> notification (payment.failed)
```

---

## Services

| Service              | gRPC  | HTTP | Database         | Notes                       |
|----------------------|-------|------|------------------|-----------------------------|
| identity-service     | —     | 8087 | Postgres + Redis | OAuth2 AS, HTTP-only        |
| user-service         | 50051 | 8081 | Postgres         | Profile only                |
| order-service        | 50053 | 8083 | Postgres         | Saga + Outbox               |
| payment-service      | 50054 | 8084 | Postgres         | Stripe                      |
| inventory-service    | 50055 | 8085 | Postgres + Redis | Stock locking               |
| product-service      | 50052 | 8082 | MongoDB  + Redis | Catalog                     |
| notification-service | —     | 8086 | MongoDB  + Redis | Kafka consumer              |
| api-gateway          | —     | 8080 | —                | HTTP proxy, rate-limiter    |
| bff-web              | —     | 8090 | Redis            | Session store, grpc-gateway |

---

## Architectural Decisions

All architectural decisions are documented as ADRs in [`docs/adr/`](docs/adr/README.md).

| ADR                                                                    | Title                                     |
|------------------------------------------------------------------------|-------------------------------------------|
| [ADR-001](docs/adr/adr-001-clean-architecture-domain-driven-design.md) | Clean Architecture + Domain-Driven Design |
| [ADR-002](docs/adr/adr-002-phase-based-graceful-shutdown.md)           | Phase-Based Graceful Shutdown             |


## Key Invariants

These must be preserved when extending the system:

1. **Domain layer has no infrastructure imports.** If `domain/*.go` imports `pgx`, `redis`, or `kafka` — it is wrong.
2. **Saga uses `orders.Update()`, never `orders.Save()`.** `Save` uses `ON CONFLICT DO NOTHING` — status transitions are silently dropped.
3. **Payment state changes only after provider succeeds.** Call `provider.Refund()` before `payment.Refund()` — never the reverse.
4. **`kid` in JWT is `SHA256(publicKey)[:8]`, never `uuid.New()`.** Random kid breaks token validation after restart.
5. **`FOR UPDATE` must be in the same transaction as `UPDATE`.** Separate calls via pgxpool use different connections — lock is released between them.
6. **`AllowAutoTopicCreation: false` in all Kafka producers.** Topics must be created explicitly with correct settings.
7. **`errors.Is(err, redis.Nil)` not `err == redis.Nil`.** Wrapped errors do not compare equal with `==`.
8. **Notification dedup is fail-open.** Redis unavailability must not block notification delivery.
9. **Internal services never call identity-service.** They trust `X-User-ID`/`X-User-Roles` headers injected by api-gateway. api-gateway is the single auth enforcement point.
10. **`MarkNotReady()` must be non-blocking.** Only `atomic.Bool.Store(false)` — no I/O, no locks. Called synchronously before all shutdown phases.
11. **OAuth2 client secret is a required env var.** Both identity-sv and bff-web panic at startup if `OAUTH2_CLIENT_SECRET` is missing — fail-fast over silent misconfiguration.
12. **Refresh token stored as `SHA256(token)`, never plaintext.** Redis compromise must not expose tokens. Secondary index `identity:refresh:index:{id}` enables `RevokeAll` on reuse detection.
13. **Delivery never imports `domain` for errors.** Usecase layer translates `domain` errors into `usecase`-level errors (`usecase/errors.go`) before returning. Delivery compares only against `usecase.ErrX` — never `domain.ErrX`.
