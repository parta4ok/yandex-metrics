# Repository Guidance

## Architecture

The project follows hexagonal architecture. Keep responsibilities separated.

| Layer | Location | Responsibility |
| --- | --- | --- |
| Domain | `internal/entities` | Domain types, invariants, and sentinel errors. Do not put configuration or framework types here. |
| Use cases | `internal/cases` | Business workflows. Depend on ports, validate domain objects, and wrap errors with operation context. |
| Ports | `internal/port` | Interfaces between use cases and adapters. |
| Adapters | `internal/adapter` | Infrastructure details such as YAML configuration and in-memory storage. |
| HTTP transport | `internal/port/http/public` | HTTP routing, request parsing, response formatting, and mapping domain errors to HTTP statuses. |
| Application | `pkg/application` | Compose adapters and use cases, then manage service lifecycle. |
| Composition root | `cmd/metrics` | Resolve configuration, construct the application, and delegate execution to it. |

Do not move infrastructure models or YAML schemas into `entities`. Configuration is
an adapter concern.

## Domain and Errors

- Prefer a rich domain model: entities own their state, invariants, and
  domain-specific behavior instead of being passive data containers.
- Keep entities focused. Do not move HTTP parsing, authorization, configuration,
  lifecycle management, or synchronization into domain types.
- Keep cross-entity workflows in use cases and infrastructure guarantees in
  adapters.
- Define reusable sentinel errors in `internal/entities/errors.go`.
- Use `github.com/pkg/errors` to wrap errors with a concrete operation context.
- Preserve the original error so callers can use `errors.Is`.
- HTTP maps `ErrInvalidParam` to `400`, `ErrNotFound` to `404`, and unexpected
  errors to `500`.
- Keep validation rules in `Metrics.Validate()`. HTTP validates transport format;
  use cases invoke domain validation; storage trusts the use-case contract.

## Metrics

- `MType` is a domain type. Add a new metric type through its typed constant and
  `MType.IsValid()`.
- `counter` requires `Delta` and accumulates it.
- `gauge` requires `Value` and replaces the stored value.
- Entity accessors do not use a `Get` prefix: `ID()`, `MType()`, `Delta()`,
  `Value()`, and `Hash()`.
- Required entity fields are set by the constructor. Optional fields use setters.

## Storage

- Every port method receives `context.Context` as its first argument and returns
  `error` as its final result.
- `inmemory.Storage` owns its state. Clone incoming mutable entities before
  storing them.
- Keep compound read-modify-write operations atomic inside storage. In particular,
  counter accumulation must happen under one exclusive lock.
- Use `sync.RWMutex`: `Lock` for writes and atomic updates, `RLock` for future
  independent reads.

## HTTP

- Use `chi` for public HTTP routes.
- Register routes in the server's private `registerRoutes` method.
- Use `http.Method*` constants for methods and constants for header names,
  media types, and URL parameter names.
- Name HTTP handler parameters `resp` (`http.ResponseWriter`) and
  `req` (`*http.Request`).
- The first-increment endpoint is:

  ```text
  POST /update/{type}/{name}/{value}
  Content-Type: text/plain
  ```

- Do not add redirect middleware.
- Use `mime.ParseMediaType` when checking `Content-Type`, so parameters such as
  `charset=utf-8` remain valid.
- `Server.Stop(ctx)` must use graceful shutdown through `http.Server.Shutdown`.
- Lifecycle contracts accept `context.Context`, even when a current adapter does
  not yet need it. The application supplies a `context.Background()`-derived
  context and configures only graceful-shutdown timeout.
- Register lifecycle components as `StartStopper` values. Start them
  concurrently, and stop all of them concurrently with one shared deadline.
- TLS is optional and is enabled through `public.WithTLS(certificateFile, keyFile)`.

## Configuration

- YAML is loaded through the config adapter using Koanf.
- The public server address is at `metrics.http.public.address`; it is a complete
  address string such as `:8080`, not a numeric port.
- TLS settings are at `metrics.http.public.tls`.
- Graceful shutdown timeout is at `metrics.application.graceful_shutdown_timeout`.
- Configuration path precedence is:

  ```text
  -config flag > METRIC_CONFIG_PATH environment variable
  ```

- Keep configuration keys in adapter constants. Do not expose Koanf outside the
  configuration adapter.

## Implementation Conventions

- Prefer clear, small methods and explicit domain branches over premature
  abstractions or patterns.
- Use a receiver for helper functions that belong to a type.
- Use generics only where the operation is genuinely type-independent; the
  pointer-copy helper in in-memory storage is an example.
- Add dependencies only when the implementation requires them.
- Run `gofmt` on changed Go files and `go test ./...` after implementation unless
  the user explicitly asks not to run tests.
