# Internal Packages

Non-public implementation. Referenced only from pkg/pyproc.

## Design Principles

- Thin pkg, thick internal: public API in pkg/, implementation details in internal/
- Interface boundaries: protocol, health, logging are independently swappable
- Error propagation: always wrap with `fmt.Errorf("context: %w", err)`

## Package Structure

```
internal/
├── protocol/   UDS communication, message serialization (msgpack)
├── health/     Health checks, automatic restart
└── logging/    Structured logging
```

## Code Conventions

- Doc comments required on all exported types/functions
- context.Context is the first parameter
- Channel operations must use select + context.Done() for cancellation
- Table-driven tests recommended

## Tests

```bash
go test -v -race ./internal/...
go test -coverprofile=coverage.out ./internal/...
```

Coverage target: 100% (CI non-blocking but aimed for)

## Review Guidelines

- Errors must be wrapped with `fmt.Errorf("context: %w", err)`
- All channel operations must be cancellable via context
- Security-sensitive code in protocol/ requires explicit approval
- No exported symbols should leak from internal/
