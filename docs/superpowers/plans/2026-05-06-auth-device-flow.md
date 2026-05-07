# Auth Device Flow Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace manual token-paste login with OAuth2 device authorization flow via Dex, with transparent token refresh on every API call, under `hlctl auth login` / `hlctl auth logout`.

**Architecture:** Discovery calls `/.well-known/homelab` on the API then `/.well-known/openid-configuration` on the Dex issuer to resolve endpoints. Tokens are persisted to `~/.config/homelab/credentials.json`. `AuthenticatedTransport` is rebuilt around `golang.org/x/oauth2` — it injects a static env token, auto-refreshes stored credentials, or passes through with no header when auth is disabled.

**Tech Stack:** Go, `golang.org/x/oauth2`, Cobra, `net/http`, `encoding/json`

---

## File Map

| File | Change |
|---|---|
| `go.mod` / `go.sum` | add `golang.org/x/oauth2` |
| `internal/config/config.go` | add `OIDCClientID` field; `ClientID()` helper |
| `internal/auth/auth.go` | extend `Credentials`; replace transport with `NewAuthenticatedTransport` + `diskSavingTokenSource` |
| `internal/auth/discover.go` | new — `DiscoverHomelab`, `DiscoverOIDC` |
| `internal/auth/deviceflow.go` | new — `Login()` device flow runner |
| `internal/cli/auth/auth.go` | new — `auth` parent + `login` + `logout` subcommands |
| `internal/cli/login/login.go` | deleted |
| `internal/cli/root.go` | swap `login.NewCmd()` → `auth.NewCmd()` |
| `internal/apiclient/apiclient.go` | use `NewAuthenticatedTransport()` (no longer an error source) |

---

## Task 1: Add `golang.org/x/oauth2` dependency

**Files:**
- Modify: `go.mod`, `go.sum`

- [ ] **Step 1: Add the dependency**

```bash
cd /path/to/homelab-cli && go get golang.org/x/oauth2@latest && go mod tidy
```

Expected: `go.mod` now lists `golang.org/x/oauth2` under `require`.

- [ ] **Step 2: Verify build still compiles**

```bash
go build ./...
```

Expected: no errors.

- [ ] **Step 3: Commit**

```bash
git add go.mod go.sum
git commit -m "chore: add golang.org/x/oauth2 dependency"
```

---

## Task 2: Extend config with `OIDCClientID`

**Files:**
- Modify: `internal/config/config.go`

- [ ] **Step 1: Add field and helper to `Config`**

Replace the `Config` struct and add a `ClientID()` method in `internal/config/config.go`:

```go
type Config struct {
	APIURL       string `yaml:"api_url"`
	OIDCClientID string `yaml:"oidc_client_id"`
}

// ClientID returns the configured OIDC client ID, defaulting to "homelab-cli".
func (c *Config) ClientID() string {
	if c.OIDCClientID != "" {
		return c.OIDCClientID
	}
	return "homelab-cli"
}
```

- [ ] **Step 2: Run existing tests to confirm nothing broke**

```bash
go test ./internal/config/...
```

Expected: PASS (or no test files — that's fine too).

- [ ] **Step 3: Commit**

```bash
git add internal/config/config.go
git commit -m "feat(config): add OIDCClientID field with default homelab-cli"
```

---

## Task 3: Rewrite `internal/auth/auth.go`

**Files:**
- Modify: `internal/auth/auth.go`

`Credentials` gains `RefreshToken`, `ClientID`, `TokenEndpoint`. The old manual `TokenValue()` and `AuthenticatedTransport` struct are replaced by `NewAuthenticatedTransport` which handles three cases: static env token, stored credentials with auto-refresh, or pass-through.

- [ ] **Step 1: Replace the entire file contents**

```go
package auth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/bwilczynski/hlctl/internal/config"
	"golang.org/x/oauth2"
)

type Credentials struct {
	AccessToken   string    `json:"access_token"`
	RefreshToken  string    `json:"refresh_token"`
	TokenType     string    `json:"token_type"`
	ExpiresAt     time.Time `json:"expires_at"`
	ClientID      string    `json:"client_id"`
	TokenEndpoint string    `json:"token_endpoint"`
}

func (c *Credentials) toOAuth2Token() *oauth2.Token {
	return &oauth2.Token{
		AccessToken:  c.AccessToken,
		RefreshToken: c.RefreshToken,
		TokenType:    c.TokenType,
		Expiry:       c.ExpiresAt,
	}
}

func credentialsFromOAuth2Token(tok *oauth2.Token, clientID, tokenEndpoint string) *Credentials {
	return &Credentials{
		AccessToken:   tok.AccessToken,
		RefreshToken:  tok.RefreshToken,
		TokenType:     tok.TokenType,
		ExpiresAt:     tok.Expiry,
		ClientID:      clientID,
		TokenEndpoint: tokenEndpoint,
	}
}

func credentialsPath() (string, error) {
	dir, err := config.Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "credentials.json"), nil
}

func LoadCredentials() (*Credentials, error) {
	path, err := credentialsPath()
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("not logged in (run 'hlctl auth login')")
		}
		return nil, err
	}

	var creds Credentials
	if err := json.Unmarshal(data, &creds); err != nil {
		return nil, fmt.Errorf("parsing credentials: %w", err)
	}
	return &creds, nil
}

func SaveCredentials(creds *Credentials) error {
	path, err := credentialsPath()
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}

	data, err := json.MarshalIndent(creds, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}

func DeleteCredentials() error {
	path, err := credentialsPath()
	if err != nil {
		return err
	}
	err = os.Remove(path)
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

// NewAuthenticatedTransport returns an http.RoundTripper that injects bearer tokens.
//
//   - HOMELAB_TOKEN env set → static token, no refresh
//   - credentials.json exists → auto-refresh via stored refresh token
//   - neither → pass-through with no Authorization header (auth disabled on server)
func NewAuthenticatedTransport(base http.RoundTripper) http.RoundTripper {
	if base == nil {
		base = http.DefaultTransport
	}

	if t := config.Token(); t != "" {
		return &staticTokenTransport{token: t, base: base}
	}

	creds, err := LoadCredentials()
	if err != nil {
		return base
	}

	cfg := &oauth2.Config{
		ClientID: creds.ClientID,
		Endpoint: oauth2.Endpoint{TokenURL: creds.TokenEndpoint},
	}
	tok := creds.toOAuth2Token()
	src := oauth2.ReuseTokenSource(tok, cfg.TokenSource(context.Background(), tok))
	saving := &diskSavingTokenSource{
		src:           src,
		clientID:      creds.ClientID,
		tokenEndpoint: creds.TokenEndpoint,
	}
	return &oauth2.Transport{Source: saving, Base: base}
}

type staticTokenTransport struct {
	token string
	base  http.RoundTripper
}

func (t *staticTokenTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req = req.Clone(req.Context())
	req.Header.Set("Authorization", "Bearer "+t.token)
	return t.base.RoundTrip(req)
}

type diskSavingTokenSource struct {
	mu            sync.Mutex
	src           oauth2.TokenSource
	lastToken     string
	clientID      string
	tokenEndpoint string
}

func (s *diskSavingTokenSource) Token() (*oauth2.Token, error) {
	tok, err := s.src.Token()
	if err != nil {
		if isExpiredRefreshToken(err) {
			return nil, fmt.Errorf("session expired (run 'hlctl auth login')")
		}
		return nil, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if tok.AccessToken != s.lastToken {
		_ = SaveCredentials(credentialsFromOAuth2Token(tok, s.clientID, s.tokenEndpoint))
		s.lastToken = tok.AccessToken
	}
	return tok, nil
}

func isExpiredRefreshToken(err error) bool {
	var re *oauth2.RetrieveError
	return errors.As(err, &re) && re.ErrorCode == "invalid_grant"
}
```

- [ ] **Step 2: Build to confirm it compiles**

```bash
go build ./internal/auth/...
```

Expected: no errors.

- [ ] **Step 3: Commit**

```bash
git add internal/auth/auth.go
git commit -m "feat(auth): rewrite transport with oauth2 auto-refresh"
```

---

## Task 4: Add `internal/auth/discover.go`

**Files:**
- Create: `internal/auth/discover.go`

- [ ] **Step 1: Write the failing tests first**

Create `internal/auth/discover_test.go`:

```go
package auth_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/bwilczynski/hlctl/internal/auth"
)

func TestDiscoverHomelab_enabled(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/.well-known/homelab" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"enabled": true, "issuer": "http://idp.example.com/dex"})
	}))
	defer srv.Close()

	info, err := auth.DiscoverHomelab(srv.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !info.Enabled {
		t.Error("expected enabled=true")
	}
	if info.Issuer != "http://idp.example.com/dex" {
		t.Errorf("unexpected issuer: %s", info.Issuer)
	}
}

func TestDiscoverHomelab_disabled(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"enabled": false})
	}))
	defer srv.Close()

	info, err := auth.DiscoverHomelab(srv.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if info.Enabled {
		t.Error("expected enabled=false")
	}
}

func TestDiscoverOIDC(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/.well-known/openid-configuration" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"device_authorization_endpoint": "http://idp.example.com/dex/device/code",
			"token_endpoint":                "http://idp.example.com/dex/token",
		})
	}))
	defer srv.Close()

	endpoints, err := auth.DiscoverOIDC(srv.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if endpoints.DeviceAuthorizationEndpoint != "http://idp.example.com/dex/device/code" {
		t.Errorf("unexpected device endpoint: %s", endpoints.DeviceAuthorizationEndpoint)
	}
	if endpoints.TokenEndpoint != "http://idp.example.com/dex/token" {
		t.Errorf("unexpected token endpoint: %s", endpoints.TokenEndpoint)
	}
}

func TestDiscoverOIDC_missingDeviceEndpoint(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"token_endpoint": "http://idp.example.com/dex/token"})
	}))
	defer srv.Close()

	_, err := auth.DiscoverOIDC(srv.URL)
	if err == nil {
		t.Fatal("expected error for missing device_authorization_endpoint")
	}
}
```

- [ ] **Step 2: Run tests — expect compile failure (types not defined yet)**

```bash
go test ./internal/auth/... 2>&1 | head -20
```

Expected: compile error mentioning `auth.DiscoverHomelab` or `auth.HomelabInfo` undefined.

- [ ] **Step 3: Create `internal/auth/discover.go`**

```go
package auth

import (
	"encoding/json"
	"fmt"
	"net/http"
)

type HomelabInfo struct {
	Enabled bool   `json:"enabled"`
	Issuer  string `json:"issuer,omitempty"`
}

type OIDCEndpoints struct {
	DeviceAuthorizationEndpoint string `json:"device_authorization_endpoint"`
	TokenEndpoint               string `json:"token_endpoint"`
}

func DiscoverHomelab(apiURL string) (*HomelabInfo, error) {
	resp, err := http.Get(apiURL + "/.well-known/homelab")
	if err != nil {
		return nil, fmt.Errorf("reaching API: %w", err)
	}
	defer resp.Body.Close()

	var info HomelabInfo
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return nil, fmt.Errorf("parsing homelab discovery: %w", err)
	}
	return &info, nil
}

func DiscoverOIDC(issuer string) (*OIDCEndpoints, error) {
	resp, err := http.Get(issuer + "/.well-known/openid-configuration")
	if err != nil {
		return nil, fmt.Errorf("reaching OIDC provider: %w", err)
	}
	defer resp.Body.Close()

	var endpoints OIDCEndpoints
	if err := json.NewDecoder(resp.Body).Decode(&endpoints); err != nil {
		return nil, fmt.Errorf("parsing OIDC configuration: %w", err)
	}
	if endpoints.DeviceAuthorizationEndpoint == "" {
		return nil, fmt.Errorf("OIDC provider does not support device authorization flow")
	}
	return &endpoints, nil
}
```

- [ ] **Step 4: Run tests — expect PASS**

```bash
go test ./internal/auth/... -run TestDiscoverHomelab -v
go test ./internal/auth/... -run TestDiscoverOIDC -v
```

Expected: all PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/auth/discover.go internal/auth/discover_test.go
git commit -m "feat(auth): add homelab and OIDC discovery"
```

---

## Task 5: Add `internal/auth/deviceflow.go`

**Files:**
- Create: `internal/auth/deviceflow.go`
- Create: `internal/auth/deviceflow_test.go`

- [ ] **Step 1: Write the failing tests**

Create `internal/auth/deviceflow_test.go`:

```go
package auth_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/bwilczynski/hlctl/internal/auth"
)

func TestLogin_success(t *testing.T) {
	pollCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/device/code":
			json.NewEncoder(w).Encode(map[string]any{
				"device_code":               "dev-code-123",
				"user_code":                 "ABCD-1234",
				"verification_uri":          "http://idp.example.com/activate",
				"verification_uri_complete": "http://idp.example.com/activate?user_code=ABCD-1234",
				"expires_in":                300,
				"interval":                  1,
			})
		case "/token":
			pollCount++
			if pollCount < 2 {
				w.WriteHeader(http.StatusBadRequest)
				json.NewEncoder(w).Encode(map[string]any{"error": "authorization_pending"})
				return
			}
			json.NewEncoder(w).Encode(map[string]any{
				"access_token":  "access-tok",
				"refresh_token": "refresh-tok",
				"token_type":    "Bearer",
				"expires_in":    3600,
			})
		}
	}))
	defer srv.Close()

	endpoints := &auth.OIDCEndpoints{
		DeviceAuthorizationEndpoint: srv.URL + "/device/code",
		TokenEndpoint:               srv.URL + "/token",
	}

	var buf bytes.Buffer
	creds, err := auth.Login(context.Background(), endpoints, "homelab-cli", &buf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if creds.AccessToken != "access-tok" {
		t.Errorf("unexpected access token: %s", creds.AccessToken)
	}
	if creds.RefreshToken != "refresh-tok" {
		t.Errorf("unexpected refresh token: %s", creds.RefreshToken)
	}
	if creds.ClientID != "homelab-cli" {
		t.Errorf("unexpected client_id: %s", creds.ClientID)
	}
	if creds.TokenEndpoint != srv.URL+"/token" {
		t.Errorf("unexpected token_endpoint: %s", creds.TokenEndpoint)
	}
	out := buf.String()
	if !strings.Contains(out, "ABCD-1234") {
		t.Errorf("expected user_code in output, got: %s", out)
	}
}

func TestLogin_accessDenied(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/device/code":
			json.NewEncoder(w).Encode(map[string]any{
				"device_code": "dev-code-123",
				"user_code":   "ABCD-1234",
				"verification_uri": "http://idp.example.com/activate",
				"verification_uri_complete": "http://idp.example.com/activate?user_code=ABCD-1234",
				"expires_in":  300,
				"interval":    1,
			})
		case "/token":
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]any{"error": "access_denied"})
		}
	}))
	defer srv.Close()

	endpoints := &auth.OIDCEndpoints{
		DeviceAuthorizationEndpoint: srv.URL + "/device/code",
		TokenEndpoint:               srv.URL + "/token",
	}

	_, err := auth.Login(context.Background(), endpoints, "homelab-cli", &bytes.Buffer{})
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "authorization denied") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestLogin_expiredToken(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/device/code":
			json.NewEncoder(w).Encode(map[string]any{
				"device_code": "dev-code-123",
				"user_code":   "ABCD-1234",
				"verification_uri": "http://idp.example.com/activate",
				"verification_uri_complete": "http://idp.example.com/activate?user_code=ABCD-1234",
				"expires_in":  300,
				"interval":    1,
			})
		case "/token":
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]any{"error": "expired_token"})
		}
	}))
	defer srv.Close()

	endpoints := &auth.OIDCEndpoints{
		DeviceAuthorizationEndpoint: srv.URL + "/device/code",
		TokenEndpoint:               srv.URL + "/token",
	}

	_, err := auth.Login(context.Background(), endpoints, "homelab-cli", &bytes.Buffer{})
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "device authorization expired") {
		t.Errorf("unexpected error: %v", err)
	}
}
```

- [ ] **Step 2: Run to confirm compile failure**

```bash
go test ./internal/auth/... -run TestLogin 2>&1 | head -10
```

Expected: compile error — `auth.Login` undefined.

- [ ] **Step 3: Create `internal/auth/deviceflow.go`**

```go
package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type deviceCodeResponse struct {
	DeviceCode              string `json:"device_code"`
	UserCode                string `json:"user_code"`
	VerificationURI         string `json:"verification_uri"`
	VerificationURIComplete string `json:"verification_uri_complete"`
	ExpiresIn               int    `json:"expires_in"`
	Interval                int    `json:"interval"`
}

type tokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
}

type tokenErrorResponse struct {
	Error string `json:"error"`
}

// Login runs the OAuth2 device authorization flow and returns credentials on success.
// User-facing instructions are written to w.
func Login(ctx context.Context, endpoints *OIDCEndpoints, clientID string, w io.Writer) (*Credentials, error) {
	dcResp, err := requestDeviceCode(ctx, endpoints.DeviceAuthorizationEndpoint, clientID)
	if err != nil {
		return nil, err
	}

	uri := dcResp.VerificationURIComplete
	if uri == "" {
		uri = dcResp.VerificationURI
	}
	fmt.Fprintf(w, "Open:  %s\nCode:  %s\nWaiting for authorization...\n", uri, dcResp.UserCode)

	interval := dcResp.Interval
	if interval == 0 {
		interval = 5
	}
	deadline := time.Now().Add(time.Duration(dcResp.ExpiresIn) * time.Second)

	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(time.Duration(interval) * time.Second):
		}

		tok, retry, err := pollToken(ctx, endpoints.TokenEndpoint, clientID, dcResp.DeviceCode, &interval)
		if err != nil {
			return nil, err
		}
		if retry {
			continue
		}

		return &Credentials{
			AccessToken:   tok.AccessToken,
			RefreshToken:  tok.RefreshToken,
			TokenType:     tok.TokenType,
			ExpiresAt:     time.Now().Add(time.Duration(tok.ExpiresIn) * time.Second),
			ClientID:      clientID,
			TokenEndpoint: endpoints.TokenEndpoint,
		}, nil
	}

	return nil, fmt.Errorf("device authorization expired")
}

func requestDeviceCode(ctx context.Context, endpoint, clientID string) (*deviceCodeResponse, error) {
	data := url.Values{
		"client_id": {clientID},
		"scope":     {"openid profile email offline_access"},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("requesting device code: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("device authorization request failed (status %d)", resp.StatusCode)
	}

	var dcResp deviceCodeResponse
	if err := json.NewDecoder(resp.Body).Decode(&dcResp); err != nil {
		return nil, fmt.Errorf("parsing device code response: %w", err)
	}
	return &dcResp, nil
}

// pollToken polls the token endpoint once. Returns (token, shouldRetry, error).
func pollToken(ctx context.Context, endpoint, clientID, deviceCode string, interval *int) (*tokenResponse, bool, error) {
	data := url.Values{
		"grant_type":  {"urn:ietf:params:oauth:grant-type:device_code"},
		"client_id":   {clientID},
		"device_code": {deviceCode},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, false, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, false, fmt.Errorf("polling token endpoint: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, false, err
	}

	if resp.StatusCode == http.StatusOK {
		var tok tokenResponse
		if err := json.Unmarshal(body, &tok); err != nil {
			return nil, false, fmt.Errorf("parsing token response: %w", err)
		}
		return &tok, false, nil
	}

	var errResp tokenErrorResponse
	if err := json.Unmarshal(body, &errResp); err != nil {
		return nil, false, fmt.Errorf("unexpected status %d", resp.StatusCode)
	}

	switch errResp.Error {
	case "authorization_pending":
		return nil, true, nil
	case "slow_down":
		*interval += 5
		return nil, true, nil
	case "expired_token":
		return nil, false, fmt.Errorf("device authorization expired")
	case "access_denied":
		return nil, false, fmt.Errorf("authorization denied")
	default:
		return nil, false, fmt.Errorf("token error: %s", errResp.Error)
	}
}
```

- [ ] **Step 4: Run tests — expect PASS**

```bash
go test ./internal/auth/... -run TestLogin -v
```

Expected: all PASS (note: `TestLogin_success` polls with interval=1s, so takes ~2 seconds).

- [ ] **Step 5: Commit**

```bash
git add internal/auth/deviceflow.go internal/auth/deviceflow_test.go
git commit -m "feat(auth): add OAuth2 device authorization flow"
```

---

## Task 6: Add `internal/cli/auth/auth.go`

**Files:**
- Create: `internal/cli/auth/auth.go`

- [ ] **Step 1: Write the failing test for logout**

Create `internal/cli/auth/auth_test.go`:

```go
package auth_test

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	authpkg "github.com/bwilczynski/hlctl/internal/auth"
	authcli "github.com/bwilczynski/hlctl/internal/cli/auth"
)

func writeTempCredentials(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	creds := authpkg.Credentials{
		AccessToken:   "tok",
		TokenType:     "Bearer",
		ExpiresAt:     time.Now().Add(time.Hour),
		ClientID:      "homelab-cli",
		TokenEndpoint: "http://localhost/token",
	}
	data, _ := json.MarshalIndent(creds, "", "  ")
	path := filepath.Join(dir, "credentials.json")
	os.WriteFile(path, data, 0o600)
	t.Setenv("HOME", dir) // config.Dir() uses os.UserHomeDir()
	return path
}

func TestLogoutCmd_deletesCredentials(t *testing.T) {
	path := writeTempCredentials(t)

	cmd := authcli.NewCmd()
	cmd.SetArgs([]string{"logout"})
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetErr(buf)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("expected credentials.json to be deleted")
	}
	if !strings.Contains(buf.String(), "Logged out") {
		t.Errorf("expected 'Logged out' in output, got: %s", buf.String())
	}
}

func TestLogoutCmd_noCredentials(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	cmd := authcli.NewCmd()
	cmd.SetArgs([]string{"logout"})
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetErr(buf)

	// logout with no credentials.json should still succeed
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(buf.String(), "Logged out") {
		t.Errorf("expected 'Logged out' in output, got: %s", buf.String())
	}
}
```

- [ ] **Step 2: Run to confirm compile failure**

```bash
go test ./internal/cli/auth/... 2>&1 | head -10
```

Expected: compile error — package `internal/cli/auth` does not exist yet.

- [ ] **Step 3: Create `internal/cli/auth/auth.go`**

```go
package auth

import (
	"context"
	"fmt"

	authpkg "github.com/bwilczynski/hlctl/internal/auth"
	"github.com/bwilczynski/hlctl/internal/cli/flags"
	"github.com/bwilczynski/hlctl/internal/config"
	"github.com/spf13/cobra"
)

func NewCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "auth",
		Short: "Authenticate with the Homelab API",
	}
	cmd.AddCommand(newLoginCmd())
	cmd.AddCommand(newLogoutCmd())
	return cmd
}

func newLoginCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "login",
		Short: "Log in via device authorization flow",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load()
			if err != nil {
				return err
			}
			apiURL := flags.GetAPIURL()
			if apiURL == "" {
				apiURL, err = cfg.ResolveAPIURL()
				if err != nil {
					return err
				}
			}

			info, err := authpkg.DiscoverHomelab(apiURL)
			if err != nil {
				return fmt.Errorf("discovery failed: %w", err)
			}
			if !info.Enabled {
				fmt.Fprintln(cmd.OutOrStdout(), "Server does not require authentication.")
				return nil
			}

			endpoints, err := authpkg.DiscoverOIDC(info.Issuer)
			if err != nil {
				return fmt.Errorf("OIDC discovery failed: %w", err)
			}

			creds, err := authpkg.Login(context.Background(), endpoints, cfg.ClientID(), cmd.OutOrStdout())
			if err != nil {
				return err
			}

			if err := authpkg.SaveCredentials(creds); err != nil {
				return fmt.Errorf("saving credentials: %w", err)
			}
			fmt.Fprintln(cmd.OutOrStdout(), "Login successful.")
			return nil
		},
	}
}

func newLogoutCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "logout",
		Short: "Remove stored credentials",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := authpkg.DeleteCredentials(); err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), "Logged out.")
			return nil
		},
	}
}
```

- [ ] **Step 4: Run tests — expect PASS**

```bash
go test ./internal/cli/auth/... -v
```

Expected: `TestLogoutCmd_deletesCredentials` and `TestLogoutCmd_noCredentials` PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/cli/auth/auth.go internal/cli/auth/auth_test.go
git commit -m "feat(cli): add auth login/logout commands"
```

---

## Task 7: Wire `auth` into `root.go`, delete `login.go`

**Files:**
- Modify: `internal/cli/root.go`
- Delete: `internal/cli/login/login.go`

- [ ] **Step 1: Update `internal/cli/root.go`**

Replace the import of `login` with `auth` and swap the `AddCommand` call:

```go
package cli

import (
	authcli "github.com/bwilczynski/hlctl/internal/cli/auth"
	"github.com/bwilczynski/hlctl/internal/cli/backups"
	"github.com/bwilczynski/hlctl/internal/cli/config"
	"github.com/bwilczynski/hlctl/internal/cli/containers"
	"github.com/bwilczynski/hlctl/internal/cli/flags"
	"github.com/bwilczynski/hlctl/internal/cli/network"
	"github.com/bwilczynski/hlctl/internal/cli/storage"
	"github.com/bwilczynski/hlctl/internal/cli/system"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "hlctl",
	Short: "CLI for controlling homelab services",
	Long:  "hlctl is a command-line interface for managing your homelab infrastructure via the Homelab API.",
}

func init() {
	rootCmd.PersistentFlags().StringVarP(&flags.OutputFormat, "output", "o", "table", "Output format: table or json")
	rootCmd.PersistentFlags().StringVar(&flags.APIURL, "api-url", "", "Override API base URL")
	rootCmd.AddCommand(authcli.NewCmd())
	rootCmd.AddCommand(backups.NewCmd())
	rootCmd.AddCommand(config.NewCmd())
	rootCmd.AddCommand(containers.NewCmd())
	rootCmd.AddCommand(network.NewCmd())
	rootCmd.AddCommand(storage.NewCmd())
	rootCmd.AddCommand(system.NewCmd())
}

func Execute() error {
	return rootCmd.Execute()
}
```

- [ ] **Step 2: Delete `internal/cli/login/login.go`**

```bash
rm internal/cli/login/login.go
```

- [ ] **Step 3: Build and run all tests**

```bash
go build ./...
go test ./...
```

Expected: build succeeds, all tests PASS.

- [ ] **Step 4: Commit**

```bash
git add internal/cli/root.go
git rm internal/cli/login/login.go
git commit -m "feat(cli): replace login command with auth subcommand group"
```

---

## Task 8: Update `apiclient.go` to use `NewAuthenticatedTransport`

**Files:**
- Modify: `internal/apiclient/apiclient.go`

The transport no longer returns an error, so `NewHTTPClient` is simplified.

- [ ] **Step 1: Replace `internal/apiclient/apiclient.go`**

```go
package apiclient

import (
	"net/http"

	"github.com/bwilczynski/hlctl/internal/auth"
	"github.com/bwilczynski/hlctl/internal/cli/flags"
	"github.com/bwilczynski/hlctl/internal/config"
)

// NewHTTPClient returns an authenticated *http.Client and the resolved API base URL.
// Precedence: --api-url flag → HOMELAB_API_URL env → config file.
func NewHTTPClient() (*http.Client, string, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, "", err
	}

	apiURL := flags.GetAPIURL()
	if apiURL == "" {
		apiURL, err = cfg.ResolveAPIURL()
		if err != nil {
			return nil, "", err
		}
	}

	httpClient := &http.Client{
		Transport: auth.NewAuthenticatedTransport(nil),
	}
	return httpClient, apiURL, nil
}
```

- [ ] **Step 2: Build and run all tests**

```bash
go build ./...
go test ./...
```

Expected: build succeeds, all tests PASS.

- [ ] **Step 3: Smoke test the binary**

```bash
go run ./cmd/hlctl auth --help
```

Expected output includes:
```
Usage:
  hlctl auth [command]

Available Commands:
  login       Log in via device authorization flow
  logout      Remove stored credentials
```

- [ ] **Step 4: Commit**

```bash
git add internal/apiclient/apiclient.go
git commit -m "feat(apiclient): use NewAuthenticatedTransport"
```
