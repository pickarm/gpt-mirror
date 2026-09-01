# PandoraHelper baseline verification

This document records the observed state of the pinned PandoraHelper baseline before GPT Mirror starts protocol refactoring or dependency modernization.

## Baseline

- Upstream: `nianhua99/PandoraHelper`
- Repository-state commit: `f6275d3dc4135a98a4e9a3957eee554d64cb4e25`
- Repository-state date: 2025-07-01
- Last observed functional non-README-only upstream commit before that state: `1186fa9869d06e79a491da06e7bf320aa5cad24d` (2024-12-25)
- Backend verification toolchain: Go 1.20.14 on Ubuntu 24.04
- Frontend verification toolchain: Node 20.x + pnpm 9 using the committed lockfile

## Verification result

| Check | Result | Notes |
| --- | --- | --- |
| Go module download | PASS | Dependencies resolve with the pinned baseline. |
| Go server build | PASS | `go build -ldflags='-s -w' ./cmd/server` succeeds. |
| Frontend frozen install | PASS | `pnpm install --frozen-lockfile` succeeds. |
| Frontend production build | PASS | `pnpm build` succeeds. |
| Linux amd64 server build | PASS | Used for Docker verification. |
| Linux arm64 server build | PASS | Used for Docker verification. |
| Upstream Dockerfile build | PASS | Image builds after producing the two binaries expected by the upstream Dockerfile. |
| Fresh SQLite startup | PASS | Service starts from a new temporary data directory. |
| `/health` probe | PASS | Running service returns a healthy response. |
| SQLite database creation | PASS | Fresh startup creates the configured database file. |
| Imported upstream test suite | FAIL (pre-existing) | The checked-in tests are stale relative to the checked-in application code; see below. |

## Pre-existing upstream test-suite breakage

The application itself builds and starts, but `go test ./test/server/...` does not compile against the pinned source tree.

Observed failures include:

- `test/server/handler/user_test.go` imports `PandoraHelper/test/mocks/service`, but that generated mock package is not present in the repository.
- `test/server/service/user_test.go` imports `PandoraHelper/test/mocks/repository`, but that generated mock package is not present in the repository.
- `test/server/repository/user_test.go` references symbols that no longer exist in the current application tree, including `repository.UserRepository`, `repository.NewUserRepository`, and `model.User`.

These failures are treated as **baseline technical debt**, not as regressions introduced by GPT Mirror. Test repair belongs in M0 CI/test work rather than the baseline import change.

## Legacy runtime behavior deliberately not fixed here

The imported application still contains obsolete third-party integration assumptions, including `oaifree`/Pandora and FuClaude-related configuration and startup URL probes. For the fresh-data startup smoke test those old proxy servers were disabled and the legacy URL probes were redirected to a local fail-fast address; no application source was changed to make the smoke test pass.

Removal of this coupling is tracked separately in M1.

## Baseline conclusion

The imported source is a usable engineering baseline because the server, frontend, Docker image, database initialization, and health path are reproducible. The stale upstream tests must be repaired before they can become a required green CI gate.
