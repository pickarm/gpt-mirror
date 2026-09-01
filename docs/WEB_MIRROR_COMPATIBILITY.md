# Official ChatGPT Web Mirror Compatibility

Status: M6 research/prototype, 2026-09-01.

## Decision

**No-go for a transparent full-site ChatGPT Web mirror as the primary product architecture.**

Use the provider-backed native UI as the stable application surface. Add an isolated persistent real-browser/session bridge only for capabilities that the Web backend binds to browser state, authentication/challenge flows, or page execution.

The read-only `internal/webmirror` prototype remains useful as a diagnostic tool for anonymous shell/static behavior and for detecting when upstream assumptions change. It is intentionally not wired into the application server.

## Why

A transparent reverse proxy is not just an HTTP forwarding problem. Current ChatGPT Web behavior spans multiple origins, browser state and long-lived transports:

- OpenAI's current network guidance lists `*.chatgpt.com`, `*.auth.openai.com`, `*.oaistatic.com`, `*.oaiusercontent.com`, `challenges.cloudflare.com`, `tcr9i.chat.openai.com` and other hosts as ChatGPT dependencies.
- The same guidance explicitly requires secure WebSocket access to `wss://ws.chatgpt.com` for ChatGPT features.
- OpenAI login troubleshooting requires JavaScript and cookies for `chatgpt.com`, `openai.com` and `auth.openai.com`, and documents Cloudflare verification loops as a browser/network-sensitive failure mode.
- Current independent Web adapters converge on a hybrid architecture: ordinary reads can use authenticated HTTP, while protected writes/login/challenge steps require a real browser or CDP-owned page. The undocumented contract changes often enough that those projects maintain dedicated compatibility/smoke layers.
- M5 already observes the same boundary: ordinary account-backed reads work through the Provider, while a send that requires browser-bound Sentinel/Arkose/Turnstile/proof state fails explicitly instead of synthesizing challenge material.

Sources used for this M6 decision:

- OpenAI Help Center, Network recommendations for ChatGPT errors on web and apps: https://help.openai.com/en/articles/9247338-network-recommendations-for-chatgpt-errors-on-web-and-apps
- OpenAI Help Center, Why can't I log in to ChatGPT?: https://help.openai.com/en/articles/7426629-why-cant-i-log-in-to-chatgpt
- OpenWeb ChatGPT adapter notes: https://github.com/openweb-org/openweb/blob/main/src/sites/chatgpt/DOC.md
- ChatGPT Web Adapter architecture/usage: https://github.com/kymuco/chatgpt-web-adapter
- Browser-backed provider example: https://github.com/guberm/chatgpt-web-provider

## Compatibility matrix

| Area | Transparent proxy | Mechanical rewrite | Real browser/session | Decision |
| --- | --- | --- | --- | --- |
| Anonymous HTML shell | Often possible | Sometimes redirects | No | Diagnostic candidate only |
| Relative static assets | Usually | No | No | Can pass through |
| Absolute/static CDN assets | Only if all external origins remain reachable | Possible but high churn | No | Keep direct external dependency; do not vendor/rewrite wholesale |
| Absolute redirects to ChatGPT | No | `Location` host rewrite | No | Prototype supports this |
| Host-scoped cookies | HTTPS mirror can receive host-only cookies | Upstream `Domain=chatgpt.com` can be made host-only | Auth semantics still browser-sensitive | Prototype rewrites Domain only |
| CSP | Sometimes | Rewriting can weaken security and breaks nonce/hash assumptions | Browser validation needed | Preserve; report incompatibility rather than deleting CSP |
| CORS / Origin / Referer | Fragile | Header rewriting is endpoint-specific | Often | Do not globally rewrite to `*` |
| Login / SSO | No | Not safely | Yes | Browser/session bridge |
| Cloudflare / anti-abuse challenge | No | No | Yes, when legitimately completed by the user/browser | Browser/session bridge; no bypass logic |
| Sentinel-protected conversation writes | No for browser-bound sessions | No | Yes | Browser/session bridge |
| SSE reads | Yes with buffering disabled | Usually no | Not inherently | Already normalized by M5 Provider |
| WebSocket | Not with an HTTP-only proxy | Dedicated Upgrade routing | Browser/client still owns connection | Explicit WS support required if ever mirrored |
| Service Worker | Origin/scope bound | Requires code/scope validation | Browser | High maintenance; blocker for transparent guarantee |
| Runtime hostname checks | No if pinned | JS rewrite required | Browser | High-churn blocker |
| File/image/media origins | Only if sibling/CDN origins remain accessible | Large rewrite surface | Sometimes | Prefer direct Provider/native UI paths |

## Prototype scope

`internal/webmirror` contains three deliberately limited pieces.

### 1. Compatibility probe

`webmirror.Probe` fetches one page without following redirects and analyzes:

- absolute redirects;
- cookie Domain/SameSite/Secure behavior;
- CSP;
- CORS origin pinning;
- absolute cross-origin asset/API references;
- runtime hostname checks;
- service-worker markers;
- WebSocket markers;
- auth/challenge markers.

It emits a machine-readable report and recommendation.

Run it manually from a network that can reach ChatGPT:

```bash
go run ./cmd/webmirror-probe \
  -upstream https://chatgpt.com/ \
  -mirror https://your-mirror.example.com/
```

The probe is intentionally observational. It does not log in, solve challenges, submit prompts or mutate the upstream account.

### 2. Safe mechanical response-header rewrite

`RewriteResponseHeaders` only:

- maps an absolute upstream `Location` back to the mirror origin;
- removes an upstream-matching Cookie `Domain` attribute so the cookie becomes host-only on the mirror.

It intentionally does **not**:

- remove or weaken CSP;
- rewrite CORS to wildcard;
- rewrite HTML/JavaScript;
- forge Origin/Referer;
- synthesize cookies/tokens;
- handle browser challenges.

### 3. Read-only reverse-proxy prototype

`NewPrototype` wraps `httputil.ReverseProxy` only for GET/HEAD compatibility experiments. It rejects:

- writes such as POST/PATCH/DELETE;
- WebSocket Upgrade requests.

This is an architectural probe, not a production mirror endpoint, and it is not registered with the main HTTP server.

## Reproducible tests

CI tests use synthetic upstream fixtures to lock the fragile assumptions without contacting ChatGPT:

- upstream absolute redirect requires rewrite;
- upstream Domain cookie becomes host-only while Secure/HttpOnly/SameSite are preserved;
- CSP and CORS are never silently weakened by the prototype;
- cross-origin asset, runtime-host, Service Worker, WebSocket and auth/challenge markers are classified;
- the probe never follows redirects;
- the prototype rejects writes and WebSocket upgrades.

## Maintenance-cost comparison

### Transparent full-site mirror

Expected maintenance burden: **very high**.

It would require continuous adaptation to:

- bundled frontend code and runtime host assumptions;
- changing CSP hashes/nonces/directives;
- cookie and auth-domain behavior;
- Cloudflare and legitimate challenge flows;
- WebSocket/SSE routing;
- service-worker scope/cache behavior;
- new sibling/CDN/media origins;
- undocumented backend and frontend deployment changes.

A partial break can be deceptive: the page may render while login, conversation send, uploads, images, notifications or account switching fail later.

### Thin native UI + Provider + browser session bridge

Expected maintenance burden: **bounded and testable**.

- Provider owns upstream conversation/model protocol and keeps stable application types.
- Native UI does not inherit ChatGPT's frontend build/CSP/service-worker churn.
- Browser bridge is isolated to browser-bound login/challenge/write behavior.
- Each layer can have fixtures and smoke tests with a clear failure class.

## Recommended architecture after M6

```text
GPT Mirror native UI
        |
        v
Service layer
        |
        v
ChatGPT Provider  ------ ordinary authenticated reads / supported mutations
        |
        +-------- Browser Session Bridge (only when browser-bound state is required)

internal/webmirror
        |
        +-------- compatibility probes / anonymous read-only experiment only
```

Do not make the full ChatGPT frontend a runtime dependency of the product. If future probes show that OpenAI makes the site genuinely origin-portable and browser challenges disappear from the required path, this decision can be revisited from evidence rather than from a permanent fork.
