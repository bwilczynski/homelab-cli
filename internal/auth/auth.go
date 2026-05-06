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
