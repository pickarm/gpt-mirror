# GPT Mirror Roadmap

## Project direction

GPT Mirror is a maintainable, self-hosted ChatGPT Web gateway. The stable product path is a thin native UI/API backed by the configured ChatGPT account; upstream conversation IDs and cloud history remain authoritative instead of being copied into an independent local chat database.

The initial generic application scaffolding was derived where appropriate from `nianhua99/PandoraHelper`. Legacy `oaifree` / `fuclaude` gateway coupling is not part of the target architecture.

Transparent reverse-proxying of the complete `chatgpt.com` website remains a separate **Experimental** track. The v1 write path instead isolates browser-sensitive create/continue operations behind a private Playwright sidecar while keeping reads/history in the lightweight Go provider.

## Engineering principles

1. **Provider isolation** — ChatGPT-specific Web behavior stays in `internal/provider/chatgpt`.
2. **Cloud conversation authority** — do not silently fork conversation ownership into a second local chat store.
3. **Transport isolation** — HTTP/SSE/proxy behavior is centralized and supports per-account routing.
4. **Browser isolation** — Playwright is a private sidecar for browser-sensitive writes, not the main application process.
5. **Credentials are secrets** — encrypted at rest, redacted in API/UI/log output, and passed to the browser worker only for an authorized write request.
6. **Compatibility is observable** — upstream breakage should surface through typed errors, tests, probes, and release gates.
7. **Docker-first** — Compose must build and start the actual release stack from source.
8. **No fake green builds** — the release gate is `go test ./...`, not a hand-picked subset.

---

# v1.0 milestones

## M0 — Reproducible baseline

**Status: complete**

- [x] Preserve upstream attribution/licensing.
- [x] Backend reproducible build.
- [x] Frontend locked-dependency build.
- [x] Fresh SQLite initialization.
- [x] CI build and smoke coverage.
- [x] Source-first Docker build.

## M1 — Remove obsolete gateway coupling

**Status: complete for active source**

- [x] Remove active `oaifree` / `fuclaude` dependency.
- [x] Add CI guard preventing legacy gateway references from returning to active source.
- [x] Move ChatGPT Web endpoint knowledge behind the provider package.

## M2 — ChatGPT provider boundary

**Status: complete for v1 core scope**

- [x] Typed `Provider` interface.
- [x] Typed provider errors.
- [x] Models and health.
- [x] Conversation list/get/create/continue.
- [x] Rename/archive/delete.
- [x] Streaming abstraction and SSE implementation.
- [x] Fake provider for tests.

## M3 — Transport and outbound proxy

**Status: complete for v1**

- [x] HTTP proxy.
- [x] HTTPS proxy.
- [x] SOCKS5.
- [x] SOCKS5H / remote DNS semantics.
- [x] Proxy authentication.
- [x] Global proxy.
- [x] Per-account proxy override.
- [x] Redacted proxy output.
- [x] Streaming-safe shared transport settings.

## M4 — Session and credential management

**Status: complete for v1 credential model**

- [x] Encrypted credential store.
- [x] Access token input.
- [x] Session token input.
- [x] Full browser Cookie input.
- [x] Credential state metadata (`healthy`, `expired`, `blocked`, `unknown`).
- [x] Provider-backed account health check.
- [x] Persist provider health result.
- [x] Redacted account APIs.
- [x] Legacy plaintext migration path.

## M5 — Conversation parity MVP

**Status: implemented; live account validation remains a release gate**

- [x] List upstream history.
- [x] Read conversation.
- [x] Create conversation.
- [x] Continue conversation.
- [x] SSE streaming.
- [x] Rename.
- [x] Archive/unarchive.
- [x] Delete.
- [x] Preserve upstream conversation IDs and cursor pagination.
- [x] Expose authenticated `/api/chatgpt/*` endpoints.
- [x] Native `/admin/chat` surface.
- [x] Native cloud-history pagination.
- [x] Destructive cloud-delete confirmation.
- [ ] Validate mirror-created conversation appears on official ChatGPT Web/app with a real authorized account.
- [ ] Validate official-Web-created conversation appears in GPT Mirror with the same account.

## M6 — Browser-backed write executor

**Status: release-candidate integration in progress**

- [x] Separate Playwright sidecar.
- [x] No published sidecar TCP port.
- [x] Go ↔ worker communication over shared Unix socket.
- [x] Worker runtime volume ownership initialized before privilege drop.
- [x] Worker executes as unprivileged `pwuser` after startup preparation.
- [x] Full cookie and per-account proxy propagation to authorized browser writes.
- [x] Real `chatgpt.com` SPA create/continue path.
- [x] Worker NDJSON → provider StreamEvent mapping.
- [x] Browser stream → synthetic upstream SSE → existing provider parser integration.
- [x] Worker protocol unit tests.
- [x] Transport integration tests.
- [x] Unix-socket `/health` probe.
- [ ] Canonical two-container Compose smoke green on the final PR head.
- [ ] Real-account browser write E2E documented for RC.
- [ ] Temporary-chat browser behavior — currently explicitly unsupported rather than silently downgraded.

## M7 — Release deployment path

**Status: implemented; browser sidecar publication validation pending**

- [x] Root source-first multi-stage server `Dockerfile`.
- [x] Root `compose.yaml` builds the current GPT Mirror stack.
- [x] First-start config initialization.
- [x] Persistent `/app/data` volume.
- [x] `/health` probe.
- [x] `/readiness` probe.
- [x] CI validates Compose config.
- [x] CI builds the canonical two-container stack.
- [x] CI preserves container logs/ps output on startup failure.
- [x] Tag workflow publishes multi-arch server image.
- [x] Tag workflow configured to publish matching browser-worker image.
- [x] SBOM/provenance enabled for release images.
- [ ] Validate both multi-arch images during RC tag publication.

## M8 — v1 release gates

**Status: release-candidate work in progress**

Required before `v1.0.0`:

- [x] `go test ./...` green after browser protocol fix on an observed PR run.
- [x] Frontend build green on an observed PR run.
- [x] Fresh SQLite startup smoke green on an observed PR run.
- [x] Architecture/secret guards green.
- [ ] Final-head canonical two-container Docker Compose smoke green.
- [ ] Final-head browser transport integration tests green.
- [ ] Real-account cloud conversation parity validation completed and documented.
- [ ] Upgrade/restart persistence smoke completed on RC stack.
- [ ] Expired/invalid session behavior validated.
- [ ] Proxy failure behavior validated.
- [ ] README/release notes reviewed against actual behavior.
- [ ] Publish `v1.0.0-rc1`.
- [ ] Validate both GHCR images from the RC tag.
- [ ] RC regression pass.
- [ ] Publish `v1.0.0`.

---

# Experimental / post-v1 tracks

## Full transparent Web mirror

The `internal/webmirror` prototype and `cmd/webmirror-probe` are compatibility research tools. They are intentionally not presented as stable 1:1 `chatgpt.com` parity.

Future work may include:

- CSP/origin/header/cookie rewrite research.
- WebSocket and service-worker behavior.
- automated upstream compatibility probes.

## Rich capabilities

Deferred until core conversation parity is stable:

- file upload/download
- image inputs/outputs
- Web/search capability adapters
- Projects/GPTs feasibility
- voice
- Deep Research
- Apps/Connectors

## Multi-account routing

Post-v1 improvements:

- account-pool health state machine
- user/account affinity
- concurrency limits
- usage metadata
- fallback policies that never silently cross conversation ownership

## Optional API compatibility

An OpenAI-compatible compatibility layer may be added later, but it must consume the same provider boundary rather than become a second independent ChatGPT implementation.

---

# Current execution order

1. Make the final browser-executor PR head fully green, including two-container Compose smoke.
2. Merge the browser-backed write executor into `main`.
3. Add automated restart/persistence/session/proxy RC regressions.
4. Run real-account conversation parity E2E without placing credentials in repository logs or fixtures.
5. Tag `v1.0.0-rc1` and validate both published GHCR images.
6. Fix RC-only defects without expanding scope.
7. Tag `v1.0.0`.
