# GPT Mirror release checklist

This checklist is the release gate for v1.x. A tag must not be promoted as stable while any required item is unresolved.

## 1. Automated gates

- [ ] `go test ./...` passes on the release commit.
- [ ] Go server build passes.
- [ ] `cmd/webmirror-probe` build passes.
- [ ] frontend installs from the committed lockfile and builds successfully.
- [ ] browser-worker protocol and write-transport integration tests pass.
- [ ] architecture-boundary checks pass.
- [ ] fresh SQLite startup smoke test passes.
- [ ] root `compose.yaml` validates with a clean environment.
- [ ] canonical two-container Compose deployment builds from source and becomes healthy.
- [ ] browser worker responds to `/health` over its private Unix socket.
- [ ] application `/health` returns healthy.
- [ ] application `/readiness` returns ready.
- [ ] `/app/data/config.json` and `/app/data/data.db` survive in the Compose volume.
- [ ] browser-worker restart recreates a healthy Unix socket.
- [ ] Go application restart becomes ready again without losing the data volume.

## 2. Credential/security gates

- [ ] new credentials cannot be persisted without a configured encryption key.
- [ ] access token, session token, cookie, password, refresh token, session key and authenticated proxy material are not present in normal account-list JSON.
- [ ] account proxy credentials are redacted in UI/API read models.
- [ ] logs do not dump complete upstream HTTP responses or credentials.
- [ ] checked-in fixtures contain only synthetic credentials.
- [ ] `.env` is ignored and `.env.example` contains placeholders only.
- [ ] legacy plaintext credential migration has backup guidance.
- [ ] browser worker has no published TCP port and no database volume.
- [ ] browser worker receives decrypted cookie/proxy material only per authorized write request.
- [ ] browser process drops to the unprivileged `pwuser` account after runtime-volume preparation.

## 3. Real-account ChatGPT parity gate

Run with an account/session that the tester is authorized to use. Record only conversation IDs, status classifications and timestamps; never attach tokens or cookies to release evidence.

- [ ] account health reports authenticated/healthy.
- [ ] model list loads.
- [ ] existing conversation history loads and preserves upstream conversation IDs.
- [ ] an existing upstream conversation opens with the expected active branch.
- [ ] GPT Mirror creates a new cloud conversation through the browser-backed write path.
- [ ] the newly created conversation is visible from the normal ChatGPT account surface.
- [ ] a conversation created outside GPT Mirror becomes visible after history refresh.
- [ ] continue streams assistant output over SSE and preserves the same conversation ID.
- [ ] rename is reflected upstream.
- [ ] archive/unarchive is reflected upstream where supported.
- [ ] delete/hide is reflected upstream.
- [ ] history pagination loads additional upstream pages without duplicate conversation IDs.
- [ ] temporary-chat behavior is verified separately; until browser support exists, the browser fallback must reject it explicitly rather than persist it silently.

## 4. Failure-mode gate

- [ ] expired/invalid credential produces an auth/expired state rather than a generic 500 where classification is possible.
- [ ] blocked/forbidden session produces a blocked/auth classification.
- [ ] HTTP 429 preserves rate-limit classification and Retry-After metadata.
- [ ] upstream protocol errors are distinguishable from transport failures.
- [ ] unreachable HTTP/SOCKS proxy fails without leaking proxy credentials.
- [ ] missing/unreachable browser-worker socket maps to an explicit unavailable error.
- [ ] malformed browser-worker protocol data maps to a protocol error.
- [ ] restarting both containers retains accounts, encrypted credentials and local metadata.
- [ ] changing/removing the credential encryption key cannot silently corrupt or plaintext-fallback existing ciphertext.
- [ ] browser-only upstream requirements are handled by the browser-backed write path when enabled; HTTP-only deployments fail explicitly instead of pretending success.

## 5. Documentation gate

- [ ] README Quick Start has been executed from a clean clone.
- [ ] `.env.example` variable names match Viper/Compose behavior.
- [ ] stable native UI/API and Experimental transparent Web mirror are clearly distinguished.
- [ ] Go application and Playwright browser-worker responsibilities are documented.
- [ ] private Unix-socket boundary is documented.
- [ ] proxy schemes and per-account override behavior are documented for both HTTP and browser writes.
- [ ] credential encryption/backup behavior is documented.
- [ ] temporary-chat browser limitation is documented while it remains unsupported.
- [ ] changelog entry exists for the tag.

## 6. Release procedure

1. Confirm the candidate commit is on `main` and every CI job is green.
2. Complete the real-account parity and failure-mode sections above.
3. Update `CHANGELOG.md` from `Unreleased` to the release version/date.
4. Create an annotated prerelease tag such as `v1.0.0-rc1`.
5. Push the tag.
6. Verify the Release workflow publishes both matching GHCR images:
   - `ghcr.io/pickarm/gpt-mirror`
   - `ghcr.io/pickarm/gpt-mirror-browser`
7. Verify SBOM/provenance metadata is attached to both image builds.
8. Pull the published RC images on a clean Docker host and repeat the two-container health/readiness/restart smoke checks.
9. Complete RC regression without expanding feature scope.
10. Update the changelog and create `v1.0.0` only after the RC gates remain green.
11. Verify both stable GHCR images and the GitHub Release before marking v1.0 as recommended stable.

## Explicit non-gate for v1.0

Full transparent reverse proxying of the entire `chatgpt.com` browser application is not a v1.0 stability requirement. The `internal/webmirror` prototype remains Experimental until browser-origin, CSP, service-worker and WebSocket behavior can be supported without weakening the stable provider/native-UI path.
