package system

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/bwilczynski/hlctl/internal/cli/cmdutil"
	gen "github.com/bwilczynski/hlctl/internal/system"
)

func okHealthResp(data gen.Health) *gen.GetSystemHealthResponse {
	b, _ := json.Marshal(data)
	return &gen.GetSystemHealthResponse{HTTPResponse: &http.Response{StatusCode: http.StatusOK}, Body: b, JSON200: &data}
}

func errHealthResp(status int, body map[string]any) *gen.GetSystemHealthResponse {
	b, _ := json.Marshal(body)
	return &gen.GetSystemHealthResponse{HTTPResponse: &http.Response{StatusCode: status}, Body: b}
}

func TestHealthCmd_tableOutput(t *testing.T) {
	msg := "disk failing"
	stub := &StubClient{
		GetSystemHealthWithResponseFunc: func(_ context.Context, _ ...gen.RequestEditorFn) (*gen.GetSystemHealthResponse, error) {
			return okHealthResp(gen.Health{
				Status:    gen.Healthy,
				CheckedAt: time.Now(),
				Components: []gen.ComponentHealth{
					{Name: "nas-1", Status: gen.Healthy},
					{Name: "unifi", Status: gen.Degraded, Message: &msg},
				},
			}), nil
		},
	}

	cmd := newHealthCmd()
	cmdutil.SetClient[SystemClient](cmd, stub)
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	for _, want := range []string{"nas-1", "healthy", "unifi", "degraded"} {
		if !strings.Contains(out, want) {
			t.Errorf("expected %q in output, got:\n%s", want, out)
		}
	}
}

func TestHealthCmd_apiError(t *testing.T) {
	stub := &StubClient{
		GetSystemHealthWithResponseFunc: func(_ context.Context, _ ...gen.RequestEditorFn) (*gen.GetSystemHealthResponse, error) {
			return errHealthResp(http.StatusUnauthorized, map[string]any{
				"type":   "https://homelab.local/problems/unauthorized",
				"title":  "Unauthorized",
				"status": 401,
				"detail": "Bearer token missing",
			}), nil
		},
	}

	cmd := newHealthCmd()
	cmdutil.SetClient[SystemClient](cmd, stub)
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "Unauthorized") {
		t.Errorf("expected 'Unauthorized' in error, got: %v", err)
	}
}
