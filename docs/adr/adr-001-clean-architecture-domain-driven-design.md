# ADR-001 - Clean Architecture + Domain-Driven Design


**Status:** Accepted

**Context:** Microservices need clear boundaries between business logic and infrastructure. Technology choices (databases, messaging) must be swappable without touching domain logic.

**Decision:** Each service follows Clean Architecture with DDD:
```
domain/                    <- domain layer (no dependencies)
  <domain>.go              <- aggregate root
  <domain>_item.go         <- entity
  money.go                 <- value object (immutable)
  address.go               <- value object (with logic)
  <domain>_status.go       <- value object / enum
  errors.go                <- domain errors
  event.go                 <- domain events
  price_calculator.go      <- domain service
usecase/                   <- application layer
  usecase.go               <- use case contracts for interface adapters layer
  <action>_<domain>.go     <- specific use case implementation
  ports.go                 <- use case contracts for frameworks and adapters layer
  dto.go                   <- input/output structs
  mapper.go                <- dto <-> domain mapping
infrastructure/            <- frameworks and drivers layer
  postgres/
    <domain>_repository.go <- repository implementation
    dto.go                 <- DB structs
    mapper.go              <- domain <-> DB mappint
delivery/                  <- interface adapters layer
  grpc/
    handler.go             <- gRPC handler
    validate.go            <- validation
    dto.go                 <- transport DTO
    mapper.go              <- grcp <-> usecase mapping
app/
  app.go                   <- composition root (manual DI, no framework)
  config/
    config.go              <- app configuration structs
```

**Key rules:**
- Output DTOs contain only primitives (`string`, `int`, `time.Time`, `bool`) - delivery layer never touches domain aggregate directly
- `mapper.go` is the single point where usecase reads domain aggregate fields and converts to DTO
- Delivery handlers don't import anything from `domain` layer
- Use case struct names differ from interface names to avoid collision: interface `CreateOrderUseCase`, struct `CreateOrder`
- `var _ Interface = (*Struct)(nil)` compile-time checks on all adapters and use case structs
- identity-sv is HTTP-only (`delivery/http/`) - internal services never call it directly

**Consequences:** More boilerplate per service. Domain layer has zero infrastructure imports - verified with compile-time checks. Delivery layer has zero domain aggregate dependencies. Use cases are fully unit-testable without infrastructure mocks.
