# Credential and session storage

Account authentication material is no longer part of the normal account-list API model. The application stores new credential material in `account_credential` as AES-256-GCM ciphertext and exposes only state metadata to services and the admin UI.

## Encryption key

Set `security.credential_key` in `data/config.json` to a 32-byte key encoded as base64 or hexadecimal.

Generate a base64 key with:

```bash
openssl rand -base64 32
```

The following forms are accepted:

```json
{
  "security": {
    "credential_key": "<base64-encoded-32-byte-key>"
  }
}
```

or an explicit prefix:

```text
base64:<value>
hex:<64-hex-characters>
```

If the setting is empty, the server can still start and accounts without new secret material can still be managed. New or replacement account secrets are rejected instead of being persisted as plaintext.

Do not lose or casually rotate this key. Existing ciphertext cannot be decrypted with a different key. Automated key rotation/keyring support is not implemented yet.

## Stored representations

The credential provider boundary supports these representations without exposing their details to AccountService:

- legacy account fields (password/session/access/refresh/session-key material)
- token credentials
- cookie credentials
- external/browser-profile references

The current admin form writes the legacy/token-compatible fields. Future ChatGPT Web implementations can add cookie or reference-based credential resolvers behind the same interface.

## Account API redaction

Normal account list/search responses contain metadata such as:

- `hasCredential`
- `credentialState`: `healthy`, `expired`, `blocked`, or `unknown`
- `credentialMessage`
- `credentialCheckedAt`
- `proxyConfigured`
- `proxyDisplay` (userinfo redacted)

They do not contain:

- password
- session token
- access token
- refresh token
- session key
- raw proxy URL credentials

`model.Account` also marks these fields as non-serializable, providing defense in depth if an internal account model is accidentally returned by a handler.

## Edit semantics

Account edit forms intentionally receive no existing secret values.

- leaving password/token/session-key fields empty preserves the current encrypted credential
- entering a non-empty secret updates encrypted credential material
- leaving the per-account proxy field empty while editing preserves the current proxy override
- the UI may show only a redacted `proxyDisplay`; that value is never written back as a proxy URL

## Legacy migration

The original account table contains plaintext-compatible columns (`password`, `session_token`, `access_token`, `refresh_token`, `session_key`). They remain in the schema temporarily for upgrade compatibility.

At startup:

1. `account_credential` is created through GORM AutoMigrate.
2. The server searches for accounts that still contain legacy secret values.
3. If `security.credential_key` is not configured, migration is skipped and a warning reports only the number of affected accounts. Secret values are not logged.
4. If the key is configured, each legacy secret is encrypted through the credential provider.
5. Existing ciphertext is successfully decrypted before legacy columns are cleared.
6. The legacy plaintext columns are cleared only after encrypted persistence succeeds.

This process is idempotent and can be completed by configuring the key and restarting a server that was previously running without one.

Before enabling migration on an existing deployment, back up the database and securely back up the credential encryption key separately.

## Session validation

Session health is represented by the `credential.Validator` interface. The default validator does not make network calls and reports `unknown` with a user-readable message. Real ChatGPT Web session validation will be implemented behind this boundary; typed errors already map expired/invalid credentials to `expired` and blocked credentials to `blocked`.
