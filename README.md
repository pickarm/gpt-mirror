# GPT Mirror

GPT Mirror is a self-hosted ChatGPT Web gateway with an account-backed native chat UI, encrypted credential storage, Docker-first deployment, and configurable HTTP/SOCKS outbound proxies.

> Unofficial project. GPT Mirror is not affiliated with or endorsed by OpenAI. ChatGPT Web is an upstream web product and its private interfaces can change without notice.

## Release status

**v1.0 release candidate work is in progress.**

The stable product path is the native GPT Mirror UI/API backed by the same ChatGPT account state. Conversations are not copied into a second local chat database: upstream conversation IDs and cloud history remain the source of truth.

The canonical Compose stack uses two isolated processes:

- `gpt-mirror`: lightweight Go application, API, admin UI, account/history reads and SSE surface.
- `browser-worker`: headed Playwright sidecar used only for browser-sensitive ChatGPT create/continue writes.

The two containers communicate over a shared Unix socket. The browser worker publishes no TCP port.

Transparent reverse-proxying of the complete `chatgpt.com` website remains **Experimental** because CSP, cookies, service workers, WebSockets, browser-origin assumptions and upstream UI changes require a different compatibility surface.

## What works

- Docker / Docker Compose deployment from source.
- Embedded React admin and native chat UI.
- ChatGPT account/session credentials stored encrypted at rest.
- Access token, session token, and full browser-cookie input.
- Account health checks against the configured ChatGPT Web provider.
- Official-account conversation list and conversation detail retrieval.
- Create and continue conversations with SSE streaming.
- Browser-backed create/continue path for sessions that require browser-executed ChatGPT Web behavior.
- Rename, archive/unarchive, and delete conversation operations.
- HTTP, HTTPS, SOCKS5, and SOCKS5H outbound proxy support.
- Global proxy plus per-account proxy override.
- Redacted account/proxy views; credential material is never returned by normal account-list APIs.
- `/health` and `/readiness` endpoints for the Go application.
- Unix-socket health probe for the browser sidecar.

## Architecture

```text
Browser
  |
  |  /admin/chat + admin console
  v
GPT Mirror (Go)
  |-- HTTP handlers / SSE
  |-- Conversation service
  |-- Account + encrypted credential store
  |-- ChatGPT provider boundary
  |-- HTTP read/history path --------------------------> chatgpt.com
  |
  `-- browser-sensitive create/continue
          |
          | shared Unix socket /run/gpt-mirror/browser.sock
          v
      Playwright browser-worker -----------------------> chatgpt.com SPA
```

The Go process remains the product/API process. The browser sidecar does not receive account secrets at startup and has no database access. Decrypted cookie/proxy material is sent to it only for an authorized write request.

ChatGPT-specific Web endpoints stay inside `internal/provider/chatgpt`. Service, repository, and UI layers consume typed provider abstractions rather than raw upstream HTTP/browser details.

## Quick start with Docker Compose

Requirements:

- Docker Engine with Compose v2
- outbound connectivity to `chatgpt.com` directly or through a configured proxy
- enough disk/RAM for the Playwright browser image when browser-backed writes are enabled

Clone the repository and create local secrets:

```bash
git clone https://github.com/pickarm/gpt-mirror.git
cd gpt-mirror
cp .env.example .env
```

Generate a credential encryption key:

```bash
openssl rand -base64 32
```

Edit `.env` and set at least:

```dotenv
ADMIN_PASSWORD=replace-with-a-strong-admin-password
SECURITY_CREDENTIAL_KEY=replace-with-the-generated-32-byte-base64-or-hex-key
CHATGPT_BROWSER_ENABLED=true
```

Optional global proxy:

```dotenv
TRANSPORT_PROXY_URL=socks5h://127.0.0.1:1080
```

Then start the canonical stack:

```bash
docker compose up -d --build
```

Open:

```text
http://localhost:9000/admin/login
```

Application health probes:

```bash
curl http://localhost:9000/health
curl http://localhost:9000/readiness
```

The root `compose.yaml` uses named volumes for `/app/data` and the private browser-worker Unix socket. The Go container initializes `/app/data/config.json` on first start and keeps SQLite data in the persistent data volume.

To inspect the two-process deployment:

```bash
docker compose ps
docker compose logs gpt-mirror browser-worker
```

## ChatGPT credentials

Add a ChatGPT account from the admin console. Supported inputs include:

- Access Token
- Session Token
- Full browser Cookie header

Secrets are written to the encrypted credential store when `security.credential_key` / `SECURITY_CREDENTIAL_KEY` is configured. Normal account search/list responses expose only metadata such as credential state and redacted proxy information.

A full browser Cookie header is the most complete input for browser-backed writes because it can preserve multiple session values such as split session cookies and `oai-did`. Session-token deployments can still be represented as a browser cookie where the upstream session accepts that form.

Do not commit real tokens, cookies, passwords, or encryption keys to this repository. `.env` is ignored; `.env.example` contains placeholders only.

## Browser-backed writes

The canonical Compose deployment enables the browser worker with:

```dotenv
CHATGPT_BROWSER_ENABLED=true
```

When browser execution is enabled and the account has usable browser-session cookie material, create/continue operations are routed through the real `chatgpt.com` browser application. Account health, model discovery and cloud-history reads continue through the lightweight HTTP provider path.

The worker:

- runs in a separate Playwright container
- has no published TCP port
- listens only on `/run/gpt-mirror/browser.sock`
- drops privileges to the unprivileged `pwuser` account after preparing the shared runtime volume
- receives cookie/proxy material only in the write request
- returns typed NDJSON events to Go, which are converted back into the provider's SSE abstraction

If the browser worker is disabled or the account has no browser cookie material, GPT Mirror retains the HTTP provider behavior rather than routing unrelated reads through Playwright.

### Current browser-write limitations

- Temporary chat is **not yet supported by the browser fallback** and returns an explicit error rather than silently changing persistence semantics.
- DOM selectors and upstream browser behavior can change; compatibility is therefore release-tested but cannot be treated as permanent.
- Full-site `chatgpt.com` mirroring is not implied by the browser write executor.

## Outbound proxy

The transport layer accepts:

```text
http://host:port
https://host:port
socks5://host:port
socks5h://host:port
```

Authenticated forms are supported as well:

```text
socks5h://username:password@host:port
```

Set a global route with `transport.proxy_url` / `TRANSPORT_PROXY_URL`, or configure a proxy on an individual account. Per-account routing takes precedence over the global proxy and is propagated to the browser worker for that account's browser write operations.

Proxy credentials are redacted from normal API/UI output.

## Native chat UI

After signing into the admin console, open:

```text
/admin/chat
```

The native chat surface can:

- select a configured ChatGPT account
- load available models
- load upstream conversation history
- open an existing cloud conversation
- create a new conversation
- continue an existing conversation
- stream assistant output over SSE
- delete and refresh conversation history

Conversation content remains upstream-owned; GPT Mirror does not maintain a second authoritative local copy.

## API surface

Authenticated ChatGPT endpoints are under `/api/chatgpt`:

```text
POST /api/chatgpt/health
POST /api/chatgpt/models
POST /api/chatgpt/conversations/list
POST /api/chatgpt/conversations/get
POST /api/chatgpt/conversations/create
POST /api/chatgpt/conversations/continue
POST /api/chatgpt/conversations/rename
POST /api/chatgpt/conversations/archive
POST /api/chatgpt/conversations/delete
```

Create/continue return server-sent events.

## Transparent Web mirror

`internal/webmirror` and `cmd/webmirror-probe` are compatibility research tools, not the stable user-facing path.

The current prototype is intentionally read-only and does not claim full transparent parity with `chatgpt.com`. Full-site mirroring would require continuously maintaining browser-origin behavior, cookie/header rewriting, challenge state, WebSockets, CSP and service-worker behavior.

## Release images

Tagged releases publish two GHCR images:

```text
ghcr.io/pickarm/gpt-mirror
ghcr.io/pickarm/gpt-mirror-browser
```

The server image and browser-worker image receive matching semantic-version tags. Release builds enable SBOM and build provenance metadata.

## Release gates

A v1.0 release must pass all of the following:

- `go test ./...`
- browser worker / Unix-socket protocol integration tests
- browser write → synthetic SSE → provider parser integration tests
- frontend locked-dependency build
- fresh SQLite startup smoke test
- root Docker Compose configuration validation
- canonical two-container Docker build/start smoke test
- application `/health` and `/readiness` probes
- browser-worker Unix-socket `/health` probe
- no legacy `oaifree`/`fuclaude` references in active source
- credential/logging architecture guards
- real-account conversation parity validation documented for the release candidate

## Project docs

- [Roadmap](ROADMAP.md)
- [Architecture](docs/ARCHITECTURE.md)
- [Credential and session storage](docs/CREDENTIALS.md)
- [Upstream/import strategy](docs/UPSTREAM.md)

## Upstream acknowledgement

The initial generic application scaffolding was evaluated and derived where appropriate from [PandoraHelper](https://github.com/nianhua99/PandoraHelper). Applicable original license and copyright notices are retained.

## Disclaimer

Use GPT Mirror only with accounts and sessions you are authorized to access. Users are responsible for complying with applicable service terms, organizational policies, and local law. Upstream ChatGPT Web behavior may change at any time, so compatibility should be treated as versioned and observable rather than permanent.
