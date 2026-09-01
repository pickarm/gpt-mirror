# Outbound Transport and Proxy Routing

`internal/transport` owns outbound HTTP client construction and proxy routing for future provider implementations.

## Routing precedence

For each account request, route selection is deterministic:

```text
per-account proxy_url > global transport.proxy_url > direct
```

A per-account override is stored in `account.proxy_url`. Leaving it empty falls back to the global transport setting.

## Supported proxy schemes

| Scheme | Authentication | DNS behavior |
| --- | --- | --- |
| `http://` | URL userinfo supported | normal HTTP proxy behavior |
| `https://` | URL userinfo supported | normal HTTPS proxy behavior |
| `socks5://` | username/password supported | target hostname is resolved locally before SOCKS connect |
| `socks5h://` | username/password supported | target hostname is sent to the SOCKS proxy for remote DNS resolution |

Examples:

```text
http://127.0.0.1:7890
http://user:password@proxy.example:8080
socks5://127.0.0.1:1080
socks5h://user:password@proxy.example:1080
```

Proxy URLs may not contain application paths, query strings, or fragments.

## Global configuration

```json
{
  "transport": {
    "proxy_url": "",
    "connect_timeout": "10s",
    "tls_handshake_timeout": "10s",
    "response_header_timeout": "30s",
    "idle_conn_timeout": "90s",
    "max_idle_conns": 100,
    "max_idle_conns_per_host": 20
  }
}
```

The normal Viper environment mapping applies, for example `TRANSPORT_PROXY_URL`.

## Streaming semantics

Clients created by the transport factory intentionally leave `http.Client.Timeout` at zero. This avoids terminating long-lived SSE or other streaming response bodies with a fixed whole-request deadline.

Instead, the transport bounds connection establishment, TLS handshake, and response-header phases. Individual provider operations should use request contexts for cancellation and operation-specific deadlines where appropriate.

## Isolation

Every `Factory.Client` call creates an isolated `http.Transport`. The code never mutates `http.DefaultTransport` or package-global proxy state, so a per-account route cannot change another account's outbound path.

## Proxy credential handling

Proxy credentials are accepted through standard URL userinfo but are never returned through `RouteInfo`. Diagnostic route metadata uses a redacted URL, and parser/validation errors are constructed from the redacted form rather than the raw proxy string.

Do not log a raw `account.proxy_url` or `transport.proxy_url` value outside the transport package.

## Health probing

`Factory.Probe` classifies failures separately:

- `route`: connection, DNS, TLS, cancellation, or route construction failure
- `proxy_auth`: HTTP `407 Proxy Authentication Required`
- `auth`: upstream `401` or `403`, typically account/session authentication
- `upstream`: other upstream HTTP errors
- `healthy`: successful route and upstream response

The probe intentionally does not parse provider-specific response bodies. Provider health logic can combine transport probe results with provider-specific session/account checks later.

## Provider integration

The ChatGPT provider should obtain its `http.Client` from this transport layer. Provider code may know ChatGPT Web endpoint paths; service/handler code must not know transport or endpoint details.
