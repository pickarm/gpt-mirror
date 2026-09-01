# Target Architecture

## Overview

GPT Mirror should separate generic application concerns from volatile ChatGPT Web behavior.

```text
                         +----------------------+
                         |      Browser/User     |
                         +-----------+----------+
                                     |
                                     v
+------------------------------------------------------------------+
|                            GPT Mirror                              |
|                                                                  |
|  +----------------+  +----------------+  +--------------------+  |
|  | Admin / Users  |  | Accounts       |  | Usage / Metadata   |  |
|  +--------+-------+  +--------+-------+  +----------+---------+  |
|           |                   |                     |            |
|           +-------------------+---------------------+            |
|                               |                                  |
|                               v                                  |
|                     +--------------------+                       |
|                     | Application Service |                       |
|                     +---------+----------+                       |
|                               |                                  |
|                               v                                  |
|                     +--------------------+                       |
|                     | ChatGPT Provider    |                       |
|                     +----+----------+----+                       |
|                          |          |                            |
|                  session |          | conversations/files/etc.   |
|                          v          v                            |
|                     +--------------------+                       |
|                     | Transport Layer     |                       |
|                     | HTTP / SSE / Proxy  |                       |
|                     +---------+----------+                       |
+-------------------------------|----------------------------------+
                                |
                 HTTP / HTTPS / SOCKS5 / SOCKS5H
                                |
                                v
                         +-------------+
                         | chatgpt.com |
                         +-------------+
```

## Architectural boundaries

### 1. Generic application layer

Should remain mostly independent of ChatGPT protocol details:

- admin authentication
- user/share-account concepts
- account persistence
- configuration
- repository/database code
- rate/concurrency policy
- audit/usage metadata
- server lifecycle

This is the primary value we expect to retain from PandoraHelper.

### 2. ChatGPT provider layer

All volatile upstream behavior belongs here.

Suggested package:

```text
internal/provider/chatgpt/
├── provider.go
├── types.go
├── errors.go
├── auth.go
├── account.go
├── models.go
├── conversations.go
├── files.go
├── images.go
└── health.go
```

Rules:

- Services must not call `chatgpt.com` directly.
- Services must not know endpoint paths such as `/backend-api/...`.
- Provider code must return typed domain errors.
- Provider implementation details should be replaceable if upstream access strategy changes.

### 3. Transport layer

Suggested package:

```text
internal/transport/
├── client.go
├── proxy.go
├── sse.go
├── retry.go
├── headers.go
└── diagnostics.go
```

Responsibilities:

- HTTP client construction
- proxy configuration
- proxy auth
- SOCKS DNS semantics
- timeouts
- streaming/SSE safety
- connection reuse
- retry rules where safe
- request IDs and redacted diagnostics

The transport layer must never log credentials, cookies, authorization headers, or full sensitive response bodies.

### 4. Session/credential adapters

Authentication methods are volatile and should not be embedded into the account service.

Conceptual boundary:

```go
type SessionProvider interface {
    Validate(ctx context.Context, credential Credential) (*SessionState, error)
    Refresh(ctx context.Context, credential Credential) (*Credential, error)
}
```

A particular adapter may eventually use cookies, a browser profile, tokens, or another mechanism, but the rest of the application should only consume normalized session state.

## Persistence model direction

PandoraHelper already has useful account/share/conversation metadata concepts. Migration should evolve rather than immediately replace those tables.

Potential account additions:

```text
provider             chatgpt
credential_type      cookie | token | browser_profile | ...
credential_ref       encrypted/local/external-secret reference
proxy_id             optional outbound proxy affinity
health_state         healthy | expired | blocked | unknown
health_checked_at
last_error_code
```

Sensitive credentials should not be returned by list APIs by default.

## Conversation ownership

Local persistence should not become a second source of truth for ChatGPT conversation content.

Preferred rule:

> Store only the metadata needed for routing, auditing and optional usage tracking; preserve upstream conversation identifiers and retrieve account-backed conversation state through the provider.

This avoids divergence between the mirror and the underlying account.

## Proxy model

Support two levels:

1. global default outbound proxy
2. per-account override

Resolution order:

```text
account proxy > global proxy > direct connection
```

Example future config:

```yaml
transport:
  proxy: socks5h://user:pass@127.0.0.1:1080

accounts:
  allow_proxy_override: true
```

Proxy secrets should be redacted in logs and admin list views.

## Web mirror strategy

The project should not assume that proxying the entire official UI will always be feasible.

Investigation areas:

- CSP
- CORS
- Origin / Referer validation
- cookies and SameSite behavior
- static asset absolute URLs
- streaming endpoints
- WebSockets
- service workers
- runtime host checks
- anti-abuse/challenge flows

Any rewriting logic must be isolated from the provider and application layers.

Suggested boundary if needed:

```text
internal/webmirror/
├── reverse.go
├── headers.go
├── cookies.go
├── htmlrewrite.go
└── compatibility.go
```

## Compatibility probes

Because ChatGPT Web is volatile, failures should be visible before users report them.

Suggested probes:

- session validation
- account metadata fetch
- models endpoint/behavior
- conversation list
- lightweight conversation read
- streaming protocol parser fixture

CI should use recorded/synthetic fixtures where live credentials are inappropriate. Runtime health probes can validate configured accounts without exposing secrets.

## Dependency modernization

Do not combine upstream import, dependency upgrade and ChatGPT protocol rewrite in one change.

Order:

1. exact baseline import
2. build verification
3. tests
4. protocol/provider refactor
5. dependency upgrades in isolated PRs

This keeps regressions attributable.
