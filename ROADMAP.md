# GPT Mirror Roadmap

## Project direction

GPT Mirror is intended to be a maintainable ChatGPT Web mirror/gateway, not a short-lived UI clone. The project will reuse mature generic scaffolding where it saves engineering effort, while isolating ChatGPT Web behavior behind replaceable provider and transport boundaries.

The initial reference implementation is `nianhua99/PandoraHelper`, primarily for its existing Go backend, account/admin flows, persistence layer, middleware structure, frontend admin console, Docker/Kubernetes deployment scaffolding, and conversation/usage metadata handling.

The legacy `oaifree` and other third-party upstream bindings are not part of the target architecture.

---

## Engineering principles

1. **Provider isolation first** — ChatGPT-specific behavior must not leak across service/repository/admin layers.
2. **Transport isolation** — outbound proxy, HTTP, SSE, cookies, and retry behavior belong in a transport layer.
3. **Baseline before modernization** — import and prove the upstream baseline before upgrading dependencies.
4. **Compatibility is observable** — upstream changes should fail through probes/tests with actionable diagnostics.
5. **Docker-first** — the default deployment path must be reproducible with Docker Compose.
6. **No giant rewrite** — preserve useful generic scaffolding; replace only obsolete coupling.
7. **Minimize bespoke Web UI** — prefer maintainable proxy/adaptation approaches over cloning every ChatGPT screen.

---

# Milestones

## M0 — Baseline import and reproducible build

**Objective:** establish a clean, attributable baseline derived from PandoraHelper without functional redesign.

Deliverables:

- [ ] Record exact PandoraHelper upstream commit used as the baseline.
- [ ] Import source while preserving upstream MIT notices.
- [ ] Confirm backend builds with the original toolchain.
- [ ] Confirm frontend builds with its original lockfile/toolchain.
- [ ] Confirm Docker image builds.
- [ ] Confirm an empty/new SQLite database can initialize.
- [ ] Document legacy dependencies and known broken integrations.
- [ ] Add a minimal CI build check.

Exit criterion:

> The repository can build and start predictably before ChatGPT-specific refactoring begins.

---

## M1 — Remove hard-coded oaifree coupling

**Objective:** separate generic application logic from the old ChatGPT integration.

Deliverables:

- [ ] Inventory all `oaifree` references.
- [ ] Inventory all legacy `Pandora` domain settings.
- [ ] Remove direct upstream-domain knowledge from service and frontend components.
- [ ] Introduce provider-neutral configuration namespaces.
- [ ] Keep migrations backward-aware where practical.

Target configuration direction:

```yaml
providers:
  chatgpt:
    enabled: true
    mode: web

transport:
  proxy: ""
```

Exit criterion:

> The application can compile without any functional dependency on `oaifree.com`.

---

## M2 — ChatGPT Provider abstraction

**Objective:** create a narrow boundary for all ChatGPT Web behavior.

Initial interface scope:

```go
type Provider interface {
    Health(ctx context.Context) error
    Account(ctx context.Context) (*AccountInfo, error)
    Models(ctx context.Context) ([]Model, error)

    ListConversations(ctx context.Context, cursor string) (*ConversationPage, error)
    GetConversation(ctx context.Context, id string) (*Conversation, error)
    CreateConversation(ctx context.Context, req CreateConversationRequest) (*Conversation, error)
    ContinueConversation(ctx context.Context, req ContinueConversationRequest) (*MessageStream, error)
    RenameConversation(ctx context.Context, id, title string) error
    ArchiveConversation(ctx context.Context, id string) error
    DeleteConversation(ctx context.Context, id string) error
}
```

Deliverables:

- [ ] `internal/provider/chatgpt` package.
- [ ] typed provider errors.
- [ ] provider health probe.
- [ ] no HTTP implementation details exposed to services.
- [ ] mock provider for unit tests.

Exit criterion:

> Business logic can operate against a mock provider without knowing how ChatGPT Web is reached.

---

## M3 — Transport and outbound proxy layer

**Objective:** centralize network behavior and make proxy routing configurable.

Required proxy schemes:

- [ ] HTTP
- [ ] HTTPS
- [ ] SOCKS5
- [ ] SOCKS5H / remote DNS semantics

Deliverables:

- [ ] `internal/transport` package.
- [ ] connection timeout / response-header timeout.
- [ ] streaming-safe transport.
- [ ] proxy authentication.
- [ ] global proxy configuration.
- [ ] per-account proxy override.
- [ ] redacted diagnostic logging.
- [ ] transport health check.

Exit criterion:

> A ChatGPT account can be bound to a deterministic outbound route without affecting other accounts.

---

## M4 — Session and credential management

**Objective:** replace legacy token-refresh coupling with a provider-owned session model.

Candidate credential modes should be treated as adapters rather than assumed permanent APIs:

- browser/session cookies
- access/session token where applicable
- persisted browser profile if a browser-backed adapter is required

Deliverables:

- [ ] credential model migration.
- [ ] encrypted-at-rest option or documented external-secret path.
- [ ] session validation.
- [ ] invalid/expired-session state reporting.
- [ ] refresh/re-auth adapter boundary.
- [ ] no credentials in application logs.

Exit criterion:

> Account health can be checked and represented without relying on `token.oaifree.com`.

---

## M5 — Conversation parity MVP

**Objective:** support the core cloud conversation lifecycle.

Required functions:

- [ ] list history
- [ ] read conversation
- [ ] create conversation
- [ ] continue conversation
- [ ] streaming response
- [ ] rename
- [ ] archive/unarchive if supported
- [ ] delete

Validation:

- [ ] conversations created through GPT Mirror can be verified against the same underlying account state where the provider permits it.
- [ ] externally created conversations can be surfaced where the provider permits it.
- [ ] pagination and conversation IDs are preserved rather than locally reinvented.

Exit criterion:

> Core chat history behaves as account-backed state rather than an independent local chat database.

---

## M6 — Web mirror layer

**Objective:** determine how much of the official Web experience can be proxied without creating an unmaintainable frontend fork.

Work items:

- [ ] inventory HTML/static asset behavior.
- [ ] inventory CSP/CORS/Origin/Host assumptions.
- [ ] inventory WebSocket/SSE endpoints.
- [ ] define header and cookie rewriting policy.
- [ ] isolate HTML/JS rewriting if unavoidable.
- [ ] detect upstream UI incompatibility automatically where possible.

Decision gate:

> If transparent Web mirroring proves substantially less reliable than a thin native UI, document the trade-off before expanding custom UI scope.

---

## M7 — Files, images, search and richer capabilities

Post-MVP capability adapters:

- [ ] file upload/download
- [ ] image inputs
- [ ] image generation outputs
- [ ] Web/search capabilities where exposed
- [ ] Projects feasibility analysis
- [ ] GPTs feasibility analysis

Explicitly deferred until core conversation parity is stable:

- Voice
- Deep Research
- Apps/Connectors
- rapidly changing experimental surfaces

---

## M8 — Multi-account routing and limits

Deliverables:

- [ ] account pool state machine.
- [ ] healthy/unhealthy account tracking.
- [ ] proxy affinity per account.
- [ ] optional user-to-account pinning.
- [ ] usage metadata.
- [ ] concurrency limits.
- [ ] graceful fallback without silently mixing conversation ownership.

---

## M9 — Optional API compatibility

Only after the Web/account path is stable:

- [ ] OpenAI-compatible `/v1/chat/completions` adapter if useful.
- [ ] Responses-style adapter feasibility.
- [ ] explicit mapping of unsupported features.

This API layer must consume the provider abstraction rather than become a second independent ChatGPT implementation.

---

# Immediate execution order

1. Import PandoraHelper at a pinned commit.
2. Prove baseline builds.
3. Add CI and smoke tests.
4. Inventory/remove oaifree coupling.
5. Add provider interface and mocks.
6. Build transport/proxy layer.
7. Implement account/session health.
8. Implement conversation lifecycle.
9. Only then attempt full Web mirroring.

---

# MVP definition

The first useful release is intentionally narrow:

- Docker Compose deployment
- admin/account management
- one or more ChatGPT accounts
- configurable outbound HTTP/SOCKS proxy
- account/session health visibility
- conversation list/get/create/continue/rename/delete
- streaming output
- stable logs and compatibility diagnostics

Everything else is secondary to this baseline.
