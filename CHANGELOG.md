# Changelog

All notable changes to GPT Mirror are documented here.

The project uses semantic versioning for stable release tags.

## [Unreleased]

_No changes yet._

## [1.0.0-rc1] - 2026-09-01

### Added

- ChatGPT Web provider boundary with typed models, conversation lifecycle, streaming and error classification.
- Authenticated `/api/chatgpt/*` endpoints for health, models, conversation history and mutation operations.
- Native `/admin/chat` UI backed by upstream ChatGPT account state, including cursor-based cloud-history pagination and destructive delete confirmation.
- SSE create/continue streaming path.
- Encrypted credential store with access-token, session-token and full-cookie inputs.
- HTTP/HTTPS/SOCKS5/SOCKS5H transport support with global and per-account routing.
- Provider-backed account health with persisted `healthy`, `expired`, `blocked` and `unknown` metadata.
- Browser-backed ChatGPT create/continue executor using an isolated headed Playwright sidecar.
- Private Go ↔ browser-worker Unix-socket protocol; the worker exposes no TCP port and receives session/proxy material only for authorized writes.
- Browser-worker protocol tests and browser-write → synthetic SSE → provider-parser integration tests.
- Browser-worker startup/health diagnostics and explicit unavailable behavior when the worker socket cannot be reached.
- Root `compose.yaml` as the canonical source-first two-container deployment path.
- `compose.registry.yaml` for matched server/browser GHCR release images.
- Container `/health` and `/readiness` probes plus browser-worker Unix-socket `/health` probe.
- Compose restart/persistence regression coverage for both processes and the application data volume.
- Architecture guards against legacy upstream coupling, secret serialization, credential-shaped fixtures and unsafe response logging.
- Fresh SQLite, full Go test, frontend build and canonical Docker Compose smoke gates in CI.
- Credential-safe `scripts/rc-live-check.sh` for repeatable authorized real-account RC validation.
- Tag-driven release automation that publishes both server and browser-worker multi-arch GHCR images with SBOM/provenance and smoke-tests the just-published image pair before creating the GitHub Release.

### Changed

- Replaced the legacy prebuilt PandoraHelper Docker deployment path with a source-first GPT Mirror stack.
- Moved active ChatGPT Web endpoint knowledge behind `internal/provider/chatgpt`.
- Browser-sensitive writes are isolated from the Go control plane instead of forcing all reads/history through Playwright.
- Conversation content is treated as upstream-owned; local persistence remains metadata-oriented rather than becoming a second authoritative chat database.
- Account read APIs expose credential/proxy state without returning raw secret fields.
- Browser-worker startup no longer depends on `xvfb-run` wrapping the Node control plane; Xvfb is managed as display infrastructure while Node owns the Unix-socket server lifecycle.
- Release image publication isolates server and browser-worker builds into independent jobs and retries a failed SBOM/provenance build once after backoff, so transient registry failures do not require lowering release attestations.

### Security

- New/replacement credential material requires a configured AES-256-GCM credential key.
- Proxy userinfo is redacted from normal UI/API diagnostics.
- The browser worker has no database mount or published TCP port.
- The browser worker drops to the unprivileged `pwuser` account after preparing the shared runtime volume.
- `.env` is ignored and release examples contain placeholders only.

### Known limitations

- ChatGPT Web is an upstream private web interface and may change without notice.
- Browser DOM selectors and upstream challenge behavior remain compatibility-sensitive even with the isolated Playwright executor.
- Browser-backed temporary chat is not yet supported and is rejected explicitly instead of silently becoming a persistent conversation.
- Full transparent `chatgpt.com` reverse proxying remains Experimental and is not the stable v1 product path.
