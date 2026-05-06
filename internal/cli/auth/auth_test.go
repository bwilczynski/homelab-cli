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
	path := filepath.Join(dir, ".config", "homelab", "credentials.json")
	os.MkdirAll(filepath.Dir(path), 0o700)
	os.WriteFile(path, data, 0o600)
	t.Setenv("HOME", dir)
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

	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(buf.String(), "Logged out") {
		t.Errorf("expected 'Logged out' in output, got: %s", buf.String())
	}
}
