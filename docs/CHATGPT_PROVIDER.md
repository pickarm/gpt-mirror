# ChatGPT Provider Boundary

`internal/provider/chatgpt` is the only application-facing abstraction for ChatGPT behavior.

## Dependency direction

```text
handlers / services
        |
        v
internal/provider/chatgpt.Provider
        |
        v
concrete provider implementation
        |
        +--> credential/session resolver
        +--> outbound transport/proxy layer
        +--> ChatGPT protocol parsing
```

Application services must not depend on ChatGPT Web endpoint paths, raw HTTP response objects, SSE parser types, cookies, browser headers, or transport-specific errors.

## Account identity and credentials

Provider methods receive an `AccountRef` containing only the local account ID. Raw access tokens, refresh tokens, cookies, session keys, and browser-profile secrets are intentionally excluded from the interface. A future concrete provider will resolve credentials through the credential/session boundary rather than accepting secrets from service code.

## Streaming

Create/continue operations return the provider-owned `Stream` interface. Consumers call `Recv(ctx)` and receive typed `StreamEvent` values. Whether the concrete implementation uses SSE, chunked HTTP, WebSocket, or another upstream mechanism is private to the provider implementation.

## Errors

Provider failures use `*chatgpt.Error` with normalized kinds:

- `auth`
- `transport`
- `rate_limit`
- `protocol`
- `not_found`
- `invalid_request`
- `unavailable`

Services should branch on `ErrorKind`, not on upstream status text or raw response bodies.

## Testing

`chatgpt.Fake` provides hook functions for unit tests without network access. `chatgpt.SliceStream` provides deterministic streaming fixtures. Any unset fake hook fails with `ErrorKindUnavailable` instead of attempting external I/O.

## Current default

Wire currently injects `UnavailableProvider`. This is intentional: introducing the provider boundary must not silently restore network access before the transport and credential layers are implemented.
