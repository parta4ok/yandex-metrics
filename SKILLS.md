# Project Skills Playbook

## Designing Domain Logic

Aim for a rich domain model:

- Put an entity's invariants and behavior that depends only on its state in the
  entity.
- Keep entity state encapsulated and prevent construction of invalid required
  state.
- Keep cross-entity orchestration in use cases.
- Keep HTTP conversion in transport adapters and atomicity, persistence, and
  synchronization in infrastructure adapters.
- Do not use a rich domain model as a reason to move every operation into an
  entity; each layer still owns its own concerns.

## Adding a Metric Type

1. Add the typed value to `entities.MType`.
2. Extend `MType.IsValid()`.
3. Add the type-specific invariant to `Metrics.Validate()`.
4. Parse the HTTP value in `Server.newMetric`.
5. Implement its storage update behavior atomically.
6. Add the required configuration and HTTP tests when tests are introduced.

## Adding an HTTP Endpoint

1. Define reusable path fragments and parameter names as constants or route vars.
2. Register the endpoint from `Server.registerRoutes` with `http.Method*`.
3. Parse only transport data in the handler.
4. Build domain entities and call `MetricServiceProvider`.
5. Return errors exclusively through `Server.handleError`.
6. Keep success and error content types aligned with the endpoint contract.

## Adding Configuration

1. Add the YAML key under the `metrics` root.
2. Add a corresponding key constant in `internal/adapter/config`.
3. Add a focused getter to the adapter.
4. Keep values transport-ready where appropriate: for example, HTTP `address`
   contains `:8080`, not only `8080`.
5. Wire the value from the composition root rather than reading configuration
   from domain, use-case, or adapter code.

## Adding a Stateful Adapter

1. Define or extend the required port in `internal/cases`.
2. Keep the use case independent from adapter implementation details.
3. Make state ownership explicit.
4. Protect shared mutable state with the appropriate synchronization primitive.
5. Put atomic compound operations in the adapter, not between multiple port calls
   in a use case.
6. Copy mutable input when the adapter retains it beyond the call.
7. Respect a cancelled context before performing work and after waiting on a lock.

## Building the Application

The future application layer should:

1. Receive configuration-derived values.
2. Construct `inmemory.Storage`.
3. Construct `cases.MetricsService` with the storage port.
4. Construct `public.Server` with `MetricServiceProvider`, address, and optional
   TLS option.
5. Register lifecycle components as `StartStopper` values.
6. Start components with an application lifecycle context.
7. Coordinate signal-driven graceful shutdown concurrently through
   `StartStopper.Stop(ctx)` with one shared deadline.

`cmd/metrics` should only resolve the configuration path, load configuration,
create the application, and delegate lifecycle management to it.
