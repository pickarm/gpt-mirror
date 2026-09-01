# GPT Mirror

> An experimental, self-hosted ChatGPT Web mirror/gateway project focused on maintainability, upstream compatibility, Docker deployment, and configurable outbound proxies.

## Status

🚧 **Planning / bootstrap stage.**

This repository is being designed as a maintainable successor-style project rather than a one-off UI clone. The initial engineering baseline will reuse the mature account/admin/database scaffolding from [nianhua99/PandoraHelper](https://github.com/nianhua99/PandoraHelper) where appropriate, while replacing its legacy `oaifree` coupling with a modular ChatGPT provider layer.

## Goals

- Preserve a mature admin/account/database foundation instead of rebuilding boilerplate.
- Isolate all ChatGPT-specific behavior behind a provider boundary.
- Support Docker-first deployment.
- Support outbound HTTP/HTTPS/SOCKS5/SOCKS5H proxies, with room for per-account proxy binding.
- Keep conversation/session handling as close to the official ChatGPT Web behavior as practical.
- Make upstream breakage diagnosable with compatibility probes and tests.
- Minimize custom UI reimplementation when the official Web experience can be proxied safely and maintainably.

## Non-goals for the first MVP

- Reimplementing every ChatGPT product surface at once.
- Building billing or commercial account-sharing features.
- Implementing Voice, Apps/Connectors, Deep Research, GPTs, and every experimental feature in the first milestone.
- Maintaining a large bespoke ChatGPT UI fork.

## Planned architecture

```text
Browser
  |
  v
GPT Mirror
  |-- Admin / users / accounts
  |-- Session & credential management
  |-- Usage / conversation metadata
  |-- Provider abstraction
  |     `-- ChatGPT Web Provider
  |-- Transport abstraction
  |     |-- HTTP/HTTPS proxy
  |     |-- SOCKS5/SOCKS5H proxy
  |     `-- SSE / streaming
  v
chatgpt.com
```

## Project docs

- [Roadmap](ROADMAP.md)
- [Target architecture](docs/ARCHITECTURE.md)
- [PandoraHelper upstream/import strategy](docs/UPSTREAM.md)

## Initial work queue

1. [#1 — Import and pin PandoraHelper baseline](../../issues/1)
2. [#2 — Add baseline CI and smoke tests](../../issues/2)
3. [#3 — Remove hard-coded oaifree/Pandora coupling](../../issues/3)
4. [#4 — Introduce ChatGPT Provider interface and mocks](../../issues/4)
5. [#5 — Build outbound transport and proxy layer](../../issues/5)
6. [#6 — Replace legacy token refresh with session/credential providers](../../issues/6)
7. [#7 — Implement account-backed conversation lifecycle](../../issues/7)
8. [#8 — Research and prototype official Web mirror compatibility](../../issues/8)

Cross-cutting work:

- [#9 — Credential and secret-handling policy](../../issues/9)
- [#10 — Dependency modernization in isolated PRs](../../issues/10)

## Upstream acknowledgement

The initial scaffolding evaluation is based on [PandoraHelper](https://github.com/nianhua99/PandoraHelper). Any imported source will preserve the applicable original license and copyright notices.

## Disclaimer

This project is experimental and unofficial. It is not affiliated with or endorsed by OpenAI. Interfaces used by ChatGPT Web may change without notice, and users are responsible for complying with applicable terms and policies.
