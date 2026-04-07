# DistroCommerce

> Production-ready distributed e-commerce platform - Go microservices, Clean Architecture, Kafka Transactional Outbox, Saga orchestration, OAuth2 AS, gRPC+Protobuf, K8s and full observability stack

[![CI](https://github.com/hzhyvinskyi/distrocommerce/actions/workflows/ci.yml/badge.svg)](https://github.com/hzhyvinskyi/distrocommerce/actions)
[![Go Report](https://goreportcard.com/badge/github.com/hzhyvinskyi/distrocommerce)](https://goreportcard.com/report/github.com/hzhyvinskyi/distrocommerce)
[![Coverage](https://codecov.io/gh/hzhyvinskyi/distrocommerce/branch/main/graph/badge.svg)](https://codecov.io/gh/hzhyvinskyi/distrocommerce)

---

## 🎯 Goal

Build a production-grade distributed system to practice every skill required for **Staff Software Engineer** level: Clean Architecture, distributed systems patterns, cloud infrastructure, observability, and operational excellence.

**Target SLAs:**
- 🚀 **1,000 RPS** steady / **5,000 RPS** burst
- ✅ **99.9% availability** (< 43 min downtime/month)
- ⚡ **p95 < 200ms**, **p99 < 500ms**

---

## 🏗️ Architecture

```
Browser -> API Gateway (:8080) -> BFF Web (:8090) -> gRPC Services

API Gateway: HTTP proxy, JWT validation (JWKS), rate limiting, circuit breakers
BFF Web:     gRPC-gateway, OAuth2 client, HttpOnly sessions (Redis), transparent token refresh
```

### Key Patterns

- **Clean Architecture** — domain, usecase, infrastructure, delivery layers. `depguard` enforces no cross-layer imports.
- **Saga Orchestration** — order creation coordinates inventory + payment with compensation.
- **Transactional Outbox + DLQ** — at-least-once delivery for domain events.
- **BFF Pattern** — JWT tokens never exposed to browser (XSS protection).
- **Polyglot Persistence** — Postgres for ACID, MongoDB for flexible schemas, Redis for caching/sessions.
- **Compile-time interface checks** — `var _ port.X = (*Impl)(nil)` in every adapter and use case.

---
