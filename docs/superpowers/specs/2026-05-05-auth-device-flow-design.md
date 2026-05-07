# Auth Device Flow Design

**Date:** 2026-05-05  
**Status:** Approved

## Overview

Replace the manual token-paste `hlctl login` command with a proper OAuth2 device authorization flow backed by Dex. Add transparent token refresh on every API call. Restructure under an `auth` subcommand group.

## Command Structure

```
hlctl auth           # parent command (no action, prints help)
hlctl auth login     # initiates device authorization flow
hlctl auth logout    # deletes ~/.config/homelab/credentials.json
```

`internal/cli/login/login.go` is deleted. `root.go` replaces `login.NewCmd()` with `auth.NewCmd()`.

## Login Flow

`hlctl auth login` executes the following steps:

1. Resolve API URL (flag → `HOMELAB_API_URL` env → `~/.config/homelab/config.yaml`)
2. `GET {apiURL}/.well-known/homelab` → `{enabled bool, issuer string}`
   - If `enabled: false` → print `Server does not require authentication.` and exit successfully (no error)
3. Read `client_id` from CLI config (`oidc_client_id`), defaulting to `"homelab-cli"`
4. `GET {issuer}/.well-known/openid-configuration` → `{device_authorization_endpoint, token_endpoint}`
5. `POST device_authorization_endpoint` with `client_id`, `scope=openid profile email offline_access` → `{device_code, user_code, verification_uri_complete, expires_in, interval}`
6. Print to user:
   ```
   Open: {verification_uri_complete}
   Code: {user_code}
   Waiting for authorization...
   ```
7. Poll `token_endpoint` every `interval` seconds:
   - `authorization_pending` → continue polling
   - `slow_down` → increase interval by 5 s, continue polling
   - `expired_token` → exit with `device authorization expired`
   - `access_denied` → exit with `authorization denied`
   - success → save tokens, print `Login successful.`

## Credentials Storage

File: `~/.config/homelab/credentials.json` (mode 0600)

```go
type Credentials struct {
    AccessToken   string    `json:"access_token"`
    RefreshToken  string    `json:"refresh_token"`
    TokenType     string    `json:"token_type"`
    ExpiresAt     time.Time `json:"expires_at"`
    ClientID      string    `json:"client_id"`      // sourced from CLI config at login time
    TokenEndpoint string    `json:"token_endpoint"` // sourced from OIDC discovery at login time
}
```

`ClientID` and `TokenEndpoint` are persisted so `AuthenticatedTransport` can refresh without re-running discovery on every API call.

## Token Refresh & Transport

`NewAuthenticatedTransport(base http.RoundTripper) http.RoundTripper`:

- If `HOMELAB_TOKEN` env is set → inject that token statically on every request (no refresh; existing behaviour)
- If `credentials.json` exists → build `oauth2.Config{ClientID, Endpoint{TokenURL}}`, create `oauth2.ReuseTokenSource`, wrap with `diskSavingTokenSource`, return `&oauth2.Transport{Source: ..., Base: base}`
- If neither → return a pass-through transport that sends requests with **no `Authorization` header** (correct when server has auth disabled)

`oauth2.ReuseTokenSource` caches the token in memory and only calls the underlying source (triggering a refresh HTTP request) when the token is within the expiry window. After a refresh, `diskSavingTokenSource` persists the new tokens to disk.

If the refresh token is itself expired, `oauth2` returns an error; the transport wraps it as:
> `session expired (run 'hlctl auth login')`

If the API returns 401 and the user has no credentials, the `apiclient` error handler surfaces:
> `not authenticated (run 'hlctl auth login')`

`apiclient.NewHTTPClient()` calls `NewAuthenticatedTransport()` — it no longer returns an error since transport construction is always successful.

## CLI Config Extension

`internal/config/config.go` — add `OIDCClientID`:

```go
type Config struct {
    APIURL       string `yaml:"api_url"`
    OIDCClientID string `yaml:"oidc_client_id"` // defaults to "homelab-cli" when empty
}
```

No new `hlctl config` subcommand needed for now — the default covers the common case. Power users can edit `~/.config/homelab/config.yaml` directly.

## Error Handling

| Situation | Message |
|---|---|
| No credentials + API returns 401 | `not authenticated (run 'hlctl auth login')` |
| Refresh token expired | `session expired (run 'hlctl auth login')` |
| Device flow timeout | `device authorization expired` |
| Device flow denied | `authorization denied` |
| Auth disabled on server (`enabled: false`) | `Server does not require authentication.` (success) |
| No credentials + auth disabled | requests proceed without Authorization header (success) |
| `hlctl auth logout` | deletes credentials.json, prints `Logged out.` |

## Files Changed

| File | Change |
|---|---|
| `internal/auth/auth.go` | extend `Credentials`, add `NewAuthenticatedTransport`, `diskSavingTokenSource` |
| `internal/auth/discover.go` | new — `DiscoverHomelab`, `DiscoverOIDC` |
| `internal/auth/deviceflow.go` | new — `Login()` device flow runner |
| `internal/cli/auth/auth.go` | new — `auth` parent + `login` + `logout` subcommands |
| `internal/cli/login/login.go` | deleted |
| `internal/cli/root.go` | swap `login.NewCmd()` → `auth.NewCmd()` |
| `internal/apiclient/apiclient.go` | use `NewAuthenticatedTransport()` |
| `internal/config/config.go` | add `OIDCClientID` field |
| `go.mod` / `go.sum` | add `golang.org/x/oauth2` |

## Dependencies

- `golang.org/x/oauth2` — device flow, token refresh, `ReuseTokenSource`, `Transport`
