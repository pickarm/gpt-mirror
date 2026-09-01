# GPT Mirror

GPT Mirror is a self-hosted ChatGPT Web gateway with an account-backed native chat UI, encrypted credential storage, Docker-first deployment, and configurable HTTP/SOCKS outbound proxies.

> Unofficial project. GPT Mirror is not affiliated with or endorsed by OpenAI. ChatGPT Web is an upstream web product and its private interfaces can change without notice.

## Release status

**v1.0 release candidate work is in progress.**

The stable product path is the native GPT Mirror UI/API backed by the same ChatGPT account state. Conversations are not copied into a second local chat database: upstream conversation IDs and cloud history remain the source of truth.

Transparent reverse-proxying of the complete `chatgpt.com` website remains **Experimental** because browser challenges, CSP, cookies, service workers, WebSockets, and other browser-origin assumptions can change independently of GPT Mirror.

## What works

- Docker / Docker Compose deployment from source.
- Embedded React admin and native chat UI.
- ChatGPT account/session credentials stored encrypted at rest.
- Access token, session token, and full browser-cookie input.
- Account health checks against the configured ChatGPT Web provider.
- Official-account conversation list and conversation detail retrieval.
- Create and continue conversations with SSE streaming.
- Rename, archive/unarchive, and delete conversation operations.
- HTTP, HTTPS, SOCKS5, and SOCKS5H outbound proxy support.
- Global proxy plus per-account proxy override.
- Redacted account/proxy views; credential material is never returned by normal account-list APIs.
- `/health` and `/readiness` endpoints for container/orchestrator probes.

## Architecture

```text
Browser
  |
  |  /admin/chat + admin console
  v
GPT Mirror
  |-- HTTP handlers / SSE
  |-- Conversation service
  |-- Account + encrypted credential store
  |-- ChatGPT provider boundary
  |-- Transport factory
  |     |-- direct
  |     |-- HTTP / HTTPS proxy
  |     `-- SOCKS5 / SOCKS5H proxy
  v
chatgpt.com
```

ChatGPT-specific Web endpoints stay inside `internal/provider/chatgpt`. Service, repository, and UI layers consume typed provider abstractions rather than raw upstream HTTP responses.

## Quick start with Docker Compose

Requirements:

- Docker Engine with Compose v2
- outbound connectivity to `chatgpt.com` directly or through a configured proxy

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
```

Optional global proxy:

```dotenv
TRANSPORT_PROXY_URL=socks5h://127.0.0.1:1080
```

Then start the release image from the repository root:

```bash
docker compose up -d --build
```

Open:

```text
http://localhost:9000/admin/login
```

Health probes:

```bash
curl http://localhost:9000/health
curl http://localhost:9000/readiness
```

The root `compose.yaml` uses a named volume for `/app/data`. The container initializes `/app/data/config.json` on first start and keeps the SQLite database in that persistent volume.

## ChatGPT credentials

Add a ChatGPT account from the admin console. Supported inputs include:

- Access Token
- Session Token
- Full browser Cookie header

Secrets are written to the encrypted credential store when `security.credential_key` / `SECURITY_CREDENTIAL_KEY` is configured. Normal account search/list responses expose only metadata such as credential state and redacted proxy information.

A full browser cookie is useful when the session depends on multiple cookie values such as split session cookies or `oai-did`. GPT Mirror can exchange an accepted ChatGPT browser session for an access token through the upstream session endpoint.

Do not commit real tokens, cookies, passwords, or encryption keys to this repository. `.env` is ignored; `.env.example` contains placeholders only.

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

Set a global route with `transport.proxy_url` / `TRANSPORT_PROXY_URL`, or configure a proxy on an individual account. Per-account routing takes precedence over the global proxy.

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

## Browser challenge limitation

Before sending a message, the Web provider checks ChatGPT's current chat requirements. If the upstream session requires Arkose, proof-of-work, Turnstile, or another browser-only challenge, GPT Mirror returns an explicit provider-unavailable error instead of pretending the request succeeded or attempting to bypass the challenge.

A browser-backed challenge executor is not part of the stable v1 path.

## Transparent Web mirror

`internal/webmirror` and `cmd/webmirror-probe` are compatibility research tools, not the stable user-facing path.

The current prototype is intentionally read-only and does not claim full transparent parity with `chatgpt.com`. Full-site mirroring would require continuously maintaining browser-origin behavior, cookie/header rewriting, challenge state, WebSockets, CSP and service-worker behavior.

## Release gates

A v1.0 release must pass all of the following:

- `go test ./...`
- frontend locked-dependency build
- fresh SQLite startup smoke test
- root Docker Compose configuration validation
- source-first Docker image build
- release-container `/health` and `/readiness` smoke test
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
