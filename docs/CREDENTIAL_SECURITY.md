# Credential and Secret-Handling Policy

This document defines the security boundary for credentials used by GPT Mirror. It applies to account authentication, browser/session material, outbound proxy credentials, external API credentials, logs, API responses, persistence, backups, tests, and operational debugging.

## Sensitive-field inventory

Treat the following values as secrets unless a narrower component explicitly proves otherwise:

- ChatGPT/OpenAI password, access token, refresh token, session token and session key.
- Raw `Cookie` / `Set-Cookie` values and browser-session cookies.
- Browser-profile or external secret references when they grant access to an authenticated profile.
- OpenAI Sentinel/chat-requirements tokens and other short-lived challenge material.
- OAuth/API keys, including OneAPI tokens and channel keys.
- HTTP/SOCKS proxy usernames and passwords. A URL containing userinfo is a secret.
- `Authorization` and `Proxy-Authorization` header values.
- `security.credential_key` and any future key-encryption key.
- `account_credential.ciphertext`. Ciphertext is not plaintext, but it is still sensitive backup material and must not be returned through normal APIs.

Email addresses, account IDs, proxy host/port, credential state and timestamps are metadata. They are not authentication secrets, but should still be handled according to the deployment's privacy requirements.

## API rules

### Secret ingress is write-only

`api/v1.AccountWriteRequest` is the explicit HTTP DTO that may accept account secret material. Secret-bearing write DTOs must never be reused as list/read response models.

`model.Account` uses `json:"-"` on credential and raw proxy fields. Admin list/search responses use `AccountSummary`, which exposes only:

- account identity/metadata;
- `hasCredential` and credential health state;
- a sanitized credential-health message;
- `proxyConfigured` and a safe proxy display value.

There is intentionally no API that reveals stored access/session/refresh tokens, cookies, passwords, authenticated proxy URLs or encrypted ciphertext. Adding a secret reveal/export API requires a separate security design and explicit authorization model.

### Error responses

Public error messages pass through `internal/security.RedactText`. Services should still prefer stable, non-secret errors; redaction is a final safety boundary, not permission to embed credentials in errors.

## Persistence rules

### Account credentials

New credential writes are persisted through `internal/provider/credential.Provider` into `account_credential.ciphertext`. Persistence is refused when `security.credential_key` is not configured.

The current cipher requires a 32-byte key. The key must be provided separately from the database and must not be committed to the repository.

### Authenticated proxy URLs

A proxy URL such as:

```text
socks5h://username:password@proxy.example:1080
```

contains a secret. GPT Mirror stores only the non-authenticated endpoint in `account.proxy_url`:

```text
socks5h://proxy.example:1080
```

The full authenticated proxy URL is stored inside the encrypted account `Secret`. At request time, the ChatGPT Provider prefers the encrypted proxy URL and falls back to the endpoint-only account value for unauthenticated proxies.

### Legacy migration

At startup, legacy plaintext account credentials are migrated into the encrypted credential store when `security.credential_key` is configured. This includes authenticated proxy userinfo. Existing encrypted credentials are merged with legacy values before the plaintext columns are cleared.

If a legacy database contains credentials but no encryption key is configured, migration does not destroy the old data. The server emits a warning and leaves those values untouched so operators can configure the key and restart. New secret-bearing account writes are still refused without a key.

A malformed legacy proxy string containing apparent userinfo (`@`) is treated as a migration error rather than silently retained as safe metadata.

## Logging and diagnostics

Never log any of the following objects wholesale:

- `AccountWriteRequest`;
- `credential.Secret`;
- `model.AccountCredential`;
- raw HTTP request/response headers;
- `resty.Request` / `resty.Response` or equivalent client objects;
- raw authenticated proxy URLs.

Use structured metadata instead: status code, account ID, provider operation, redacted proxy endpoint and typed error kind.

`internal/security` is the centralized redaction boundary:

- `RedactURL` removes URL userinfo, sensitive query values and fragments.
- `RedactHeaders` replaces sensitive header values wholesale.
- `RedactText` scrubs common Bearer/Basic/JWT/API-key/assignment/URL-userinfo forms.
- `IsSensitiveKey` is the canonical key/header classification helper.

`internal/transport.RedactProxyURL` delegates to this shared implementation.

Redaction helpers are best-effort protection for diagnostics. The primary rule remains: do not pass secret-bearing structures to a logger.

## Credential health messages

Validator health messages can be persisted and later exposed through `AccountSummary.CredentialMessage`. They are therefore sanitized before persistence and sanitized again on read. Validators should still return categorical messages rather than raw upstream headers, cookies or bodies.

## Backups and exports

Database backups may contain encrypted credentials and must be protected as sensitive material.

- Back up `security.credential_key` separately from database backups.
- Possession of both the database and its credential key is equivalent to possession of the stored secrets.
- Do not place the credential key inside the same archive, object-store prefix or source-control repository as the database backup unless the backup system provides an independent encryption/access-control layer.
- Normal application exports must exclude `account_credential.ciphertext` and all secret values.
- There is no supported plaintext secret-export operation.

### Key rotation

Online credential-key rotation/re-encryption is not implemented. Replacing `security.credential_key` without re-encrypting stored rows makes existing ciphertext unreadable. Do not rotate the key by simply changing configuration. A future rotation feature must decrypt with the old key, re-encrypt atomically with the new key, verify every row, and define rollback semantics.

## Incident response

If a credential may have appeared in logs, an API response, a leaked backup or source control:

1. Treat it as compromised; redacting/deleting the historical copy is not sufficient.
2. Revoke or rotate the upstream credential (session/token/API key/proxy password).
3. Remove the leaked material from active logs/artifacts and restrict access to historical copies.
4. Review adjacent credentials from the same account/profile and invalidate them when appropriate.
5. Add a regression test or CI guard for the leak path before closing the incident.

## Test and CI policy

Tests use synthetic short credentials only. They must never contain real account tokens, production session cookies, real proxy credentials or copied browser-profile secrets.

CI performs high-confidence scans for production-shaped credentials in `test/` and `data/`, and architecture checks prevent secret fields from becoming JSON-readable. Security tests additionally verify:

- Account JSON never serializes secret values;
- ciphertext does not contain plaintext credentials;
- credential health messages are sanitized before persistence/read;
- public error responses redact secret-shaped values;
- authenticated proxy URLs expose only safe endpoint metadata outside the encrypted secret;
- legacy migration removes plaintext proxy userinfo without overwriting existing encrypted credentials.

## Review checklist for future changes

Before merging a change that touches authentication, networking or account APIs, verify:

1. Does any new value authenticate a user, account, browser profile, upstream service or proxy? If yes, classify it as secret by default.
2. Is it stored only through the encrypted credential boundary or an external secret reference?
3. Can any list/read/API serializer expose it?
4. Can an error, health message or HTTP client object carry it into logs?
5. Are URLs and headers passed through centralized redaction before diagnostics?
6. Are backups/export semantics explicit?
7. Is there a regression test for the new secret path?
