# GPT Mirror release checklist

This checklist is the release gate for v1.x. A tag must not be promoted as stable while any required item is unresolved.

## 1. Automated gates

- [ ] `go test ./...` passes on the release commit.
- [ ] Go server build passes.
- [ ] `cmd/webmirror-probe` build passes.
- [ ] frontend installs from the committed lockfile and builds successfully.
- [ ] architecture-boundary checks pass.
- [ ] fresh SQLite startup smoke test passes.
- [ ] root `compose.yaml` validates with a clean environment.
- [ ] root Compose deployment builds from source and becomes healthy.
- [ ] `/health` returns healthy.
- [ ] `/readiness` returns ready.
- [ ] `/app/data/config.json` and `/app/data/data.db` survive in the Compose volume.

## 2. Credential/security gates

- [ ] new credentials cannot be persisted without a configured encryption key.
- [ ] access token, session token, cookie, password, refresh token, session key and authenticated proxy material are not present in normal account-list JSON.
- [ ] account proxy credentials are redacted in UI/API read models.
- [ ] logs do not dump complete upstream HTTP responses or credentials.
- [ ] checked-in fixtures contain only synthetic credentials.
- [ ] `.env` is ignored and `.env.example` contains placeholders only.
- [ ] legacy plaintext credential migration has backup guidance.

## 3. Real-account ChatGPT parity gate

Run with an account/session that the tester is authorized to use. Record only IDs/statuses; never attach tokens or cookies to the release evidence.

- [ ] account health reports authenticated/healthy.
- [ ] model list loads.
- [ ] existing conversation history loads and preserves upstream conversation IDs.
- [ ] an existing upstream conversation opens with the expected active branch.
- [ ] GPT Mirror creates a new cloud conversation.
- [ ] the newly created conversation is visible from the normal ChatGPT account surface.
- [ ] a conversation created outside GPT Mirror becomes visible after history refresh.
- [ ] continue streams assistant output over SSE and preserves the same conversation ID.
- [ ] rename is reflected upstream.
- [ ] archive/unarchive is reflected upstream where supported.
- [ ] delete/hide is reflected upstream.
- [ ] temporary-chat behavior is verified separately and is not mistaken for persistent history.

## 4. Failure-mode gate

- [ ] expired/invalid credential produces an auth/expired state rather than a generic 500 where classification is possible.
- [ ] blocked/forbidden session produces a blocked/auth classification.
- [ ] HTTP 429 preserves rate-limit classification and Retry-After metadata.
- [ ] upstream protocol errors are distinguishable from transport failures.
- [ ] unreachable proxy/upstream fails without leaking proxy credentials.
- [ ] restarting the container retains accounts, encrypted credentials and local metadata.
- [ ] changing/removing the credential encryption key cannot silently corrupt or plaintext-fallback existing ciphertext.
- [ ] browser challenge requirements return the explicit browser-executor-unavailable error.

## 5. Documentation gate

- [ ] README Quick Start has been executed from a clean clone.
- [ ] `.env.example` variable names match Viper/Compose behavior.
- [ ] stable native UI/API and Experimental transparent Web mirror are clearly distinguished.
- [ ] proxy schemes and per-account override behavior are documented.
- [ ] credential encryption/backup behavior is documented.
- [ ] known upstream browser-challenge limitation is documented.
- [ ] changelog entry exists for the tag.

## 6. Release procedure

1. Confirm the candidate commit is on `main` and CI is green.
2. Complete the real-account parity and failure-mode sections above.
3. Update `CHANGELOG.md` from `Unreleased` to the release version/date.
4. Create an annotated version tag, for example `v1.0.0`.
5. Push the tag.
6. Verify the Release workflow publishes the GHCR image and GitHub Release.
7. Pull the published image on a clean Docker host and repeat `/health` + `/readiness` smoke checks.
8. Only then mark the GitHub Release as the recommended stable release.

## Explicit non-gate for v1.0

Full transparent reverse proxying of the entire `chatgpt.com` browser application is not a v1.0 stability requirement. The `internal/webmirror` prototype remains Experimental until browser-origin, challenge, CSP, service-worker and WebSocket behavior can be supported without weakening the stable provider/native-UI path.
