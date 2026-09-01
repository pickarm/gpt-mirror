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
WebProvider
        |
        +--> encrypted credential/session resolver
        +--> per-account outbound transport/proxy layer
        +--> ChatGPT Web protocol + SSE parsing
```

Application services must not depend on ChatGPT Web endpoint paths, raw HTTP response objects, SSE parser types, cookies, Sentinel headers, browser headers, or transport-specific errors.

## Account identity, credentials, and routing

Provider methods receive an `AccountRef` containing only the local account ID. `WebProvider` resolves the account through the encrypted credential provider introduced in M4.

Supported credential forms are:

- an encrypted ChatGPT access token; or
- an encrypted browser-session cookie/session token, exchanged through `/api/auth/session` for an access token.

Outbound HTTP clients are created by the M3 transport factory. Routing priority remains account proxy, then global `transport.proxy_url`, then direct connection. Credential material and raw proxy credentials are never included in Provider errors.

## Conversation lifecycle

`WebProvider` implements the Provider lifecycle:

- list conversation history with pagination;
- read one upstream conversation;
- create a conversation;
- continue a conversation;
- stream normalized response events;
- rename;
- archive/unarchive;
- delete/hide.

ChatGPT conversation and message identifiers are preserved verbatim. Conversation detail is a tree upstream; the parser follows `current_node` backwards through each node's `parent`, reverses that path, and exposes only the active branch. Alternate branches and visually hidden messages are not silently merged into the transcript.

### Pagination

The public API exposes an opaque `PageRequest.Cursor` rather than leaking Web-backend pagination details.

The current backend commonly uses `offset` + `limit`. In that form the Provider emits internal `offset:N` cursors. If an upstream response supplies an opaque `next_cursor`, the Provider preserves it and sends it back through `cursor=` on the next call. Callers therefore do not need to know which pagination form the upstream currently uses.

## Streaming

Create/continue operations return the provider-owned `Stream` interface. The Web implementation consumes SSE and emits:

- `conversation` when the upstream conversation ID first appears;
- `message_delta` for newly appended assistant text;
- `message_completed` with the authoritative complete assistant message;
- `done` for `[DONE]`.

ChatGPT Web often emits cumulative assistant text rather than literal deltas, so the parser calculates the newly-added suffix. If an in-progress block is rewritten, `message_completed` remains the authoritative final snapshot.

Malformed SSE JSON and upstream stream errors become `ErrorKindProtocol`; they are not converted into successful completion.

## Sentinel and browser-bound challenges

Before sending, `WebProvider` requests `/backend-api/sentinel/chat-requirements` and forwards its requirements token when available.

The Go Provider intentionally does **not** synthesize or bypass Arkose, Turnstile, proof-of-work, Cloudflare, or other anti-abuse challenges. If the authenticated account requires a browser-bound challenge, create/continue returns `ErrorKindUnavailable` indicating that a browser executor is required.

Browser/session challenge execution belongs to the later Web-mirror compatibility layer rather than the core conversation protocol adapter.

## Local data ownership

ChatGPT remains the source of truth for conversation content and message history.

The active GORM model now maps to `conversation_metadata`, storing only:

- local account ID;
- upstream conversation ID;
- upstream message ID;
- model;
- operation;
- timestamp.

The legacy `conversations` table is not automatically dropped because existing installations may contain historical data, but new code no longer uses it as the active model and does not persist user/assistant bodies locally.

## Errors

Provider failures use `*chatgpt.Error` with normalized kinds:

- `auth`: HTTP 401/403 or unavailable account credentials;
- `transport`: network failures and upstream 5xx responses;
- `rate_limit`: HTTP 429, including parsed `Retry-After` seconds;
- `protocol`: malformed upstream JSON/SSE or unexpected protocol state;
- `not_found`: HTTP 404;
- `invalid_request`: local validation and HTTP 400/422;
- `unavailable`: a required execution capability is not configured, such as browser challenge handling.

Services should branch on `ErrorKind`, not on upstream status text or raw response bodies.

## Configuration

```json
{
  "chatgpt": {
    "base_url": "https://chatgpt.com",
    "conversation_path": "/backend-api/f/conversation",
    "user_agent": ""
  }
}
```

`user_agent` may be empty to use the built-in browser-like default. `conversation_path` is restricted to `/backend-api/*` so configuration cannot accidentally route conversation sends to an unrelated path.

## Testing

`chatgpt.Fake` and `chatgpt.SliceStream` remain available for service tests. M5 also uses `httptest` against the public Provider surface to cover pagination, active-branch parsing, typed HTTP errors, Sentinel challenge behavior, and fixture-based SSE normalization without contacting ChatGPT.
