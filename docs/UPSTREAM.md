# PandoraHelper Upstream Strategy

## Why use PandoraHelper as the starting baseline

PandoraHelper already contains substantial generic infrastructure that would otherwise need to be rebuilt:

- Go backend application structure
- Gin HTTP server
- account management
- share/user management
- SQLite/MySQL/PostgreSQL persistence support
- repository/service/handler layering
- admin frontend
- scheduled tasks
- usage/conversation metadata hooks
- Docker and Kubernetes deployment examples
- configuration and logging

The goal is to reuse this generic foundation, not to preserve its legacy ChatGPT integration unchanged.

## Known legacy coupling to remove

The current upstream contains direct dependencies on services such as:

```text
https://chat.oaifree.com
https://token.oaifree.com
https://new.oaifree.com
```

Known coupling areas include:

- default configuration
- account token refresh service
- reverse proxy server
- frontend links/placeholders
- deployment configuration
- legacy naming (`Pandora`, `oaifree`)

These should be inventoried and replaced behind provider/session abstractions.

## Import policy

The first source import should be intentionally boring.

1. Pin one exact upstream commit.
2. Import that tree with minimal edits.
3. Preserve original MIT license/copyright notices.
4. Add `THIRD_PARTY_NOTICES.md` identifying the baseline and commit.
5. Confirm backend/frontend/Docker builds.
6. Only after the baseline is reproducible, begin refactoring.

Do **not** combine the initial import with:

- Go major/minor modernization
- frontend framework upgrades
- database redesign
- mass renaming
- new ChatGPT protocol implementation
- large formatting changes

## Retain / refactor / remove matrix

| Area | Initial action | Rationale |
|---|---|---|
| `internal/repository` | Retain | Generic persistence foundation |
| `internal/model` | Retain + evolve | Useful account/share metadata; schema will need additions |
| `internal/handler` | Retain + refactor | Generic API/admin handlers are reusable |
| `internal/middleware` | Retain selectively | Logging/rate/moderation concepts reusable; inspect for legacy assumptions |
| `internal/service` | Retain + decouple | Business logic useful, but account refresh is coupled to old upstream |
| `internal/server` | Refactor | Current ChatGPT reverse proxy targets legacy upstream |
| `frontend` admin pages | Retain + clean | Useful management UI; remove legacy links/text/fields as needed |
| `pkg/config` | Refactor | Remove hard-coded legacy domains, add provider/transport config |
| scheduled refresh task | Retain concept | Move implementation behind session provider |
| `oaifree` integration | Remove | Not part of target architecture |
| `fuclaude` integration | Remove or isolate | Out of initial GPT Mirror scope |
| bespoke old ChatGPT assumptions | Remove | Must not constrain new provider design |

## First expected refactor seams

### Reverse proxy

Current pattern conceptually resembles:

```go
target := conf.GetString("http.proxy-pass.oaifree.host")
proxy := httputil.NewSingleHostReverseProxy(target)
```

This should eventually become either:

```text
WebMirror -> upstream transport
```

or

```text
Application service -> ChatGPT Provider -> transport
```

with no `oaifree` knowledge in the server layer.

### Account refresh

Legacy refresh logic calls a configured token domain directly.

Target:

```text
AccountService
    -> SessionProvider
        -> credential/session implementation
```

The account service should only understand normalized health/credential state.

## Naming strategy

Avoid a giant rename immediately after import. Rename incrementally after tests exist.

Preferred target names:

- project/module: `gpt-mirror`
- product name: `GPT Mirror`
- provider: `chatgpt`
- legacy names should be removed as touched rather than through an unauditable global replacement.

## License handling

PandoraHelper is MIT-licensed. Any copied source must retain the applicable copyright and permission notice.

GPT Mirror may add its own copyright statement for new work while preserving upstream notices in copied/substantial portions.

## Upstream sync policy

PandoraHelper is being used as a baseline, not as a permanently mergeable upstream.

After the baseline import:

- upstream fixes may be cherry-picked manually when relevant;
- ChatGPT integration changes from upstream should not be assumed compatible;
- GPT Mirror-specific provider/transport boundaries take precedence over maintaining merge compatibility.

This keeps the project maintainable instead of permanently constrained by legacy architecture.
