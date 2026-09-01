# Changelog

All notable changes to GPT Mirror are documented here.

The project uses semantic versioning for stable release tags.

## [Unreleased]

### Added

- ChatGPT Web provider boundary with typed models, conversation lifecycle, streaming and error classification.
- Authenticated `/api/chatgpt/*` endpoints for health, models, conversation history and mutation operations.
- Native `/admin/chat` UI backed by upstream ChatGPT account state.
- SSE create/continue streaming path.
- Encrypted credential store with access-token, session-token and full-cookie inputs.
- HTTP/HTTPS/SOCKS5/SOCKS5H transport support with global and per-account routing.
- Provider-backed account health with persisted `healthy`, `expired`, `blocked` and `unknown` metadata.
- Root `compose.yaml` and `.env.example` as the canonical source-first Docker deployment path.
- Container `/health` and `/readiness` probes.
- Architecture guards against legacy upstream coupling, secret serialization, credential-shaped fixtures and unsafe response logging.
- Fresh SQLite, full Go test, frontend build and canonical Docker Compose smoke gates in CI.
- Release checklist and tag-driven release automation.

### Changed

- Replaced the legacy prebuilt PandoraHelper Docker deployment path with a source-first GPT Mirror image.
- Moved active ChatGPT Web endpoint knowledge behind `internal/provider/chatgpt`.
- Conversation content is treated as upstream-owned; local persistence remains metadata-oriented rather than becoming a second authoritative chat database.
- Account read APIs expose credential/proxy state without returning raw secret fields.

### Security

- New/replacement credential material requires a configured AES-256-GCM credential key.
- Proxy userinfo is redacted from normal UI/API diagnostics.
- `.env` is ignored and release examples contain placeholders only.

### Known limitations

- ChatGPT Web is an upstream private web interface and may change without notice.
- Sessions that require Arkose, Turnstile, proof-of-work or another browser-only challenge return an explicit unavailable error; GPT Mirror does not bypass those challenges.
- Full transparent `chatgpt.com` reverse proxying remains Experimental and is not the stable v1 product path.
