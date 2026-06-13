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
	systemapi "github.com/bwilczynski/hlctl/internal/api/system"
)

func okHealthResp(data systemapi.Health) *systemapi.GetSystemHealthResponse {
	b, _ := json.Marshal(data)
	return &systemapi.GetSystemHealthResponse{HTTPResponse: &http.Response{StatusCode: http.StatusOK}, Body: b, JSON200: &data}
}

func errHealthResp(status int, body map[string]any) *systemapi.GetSystemHealthResponse {
	b, _ := json.Marshal(body)
	return &systemapi.GetSystemHealthResponse{HTTPResponse: &http.Response{StatusCode: status}, Body: b}
}

func TestHealthCmd_tableOutput(t *testing.T) {
	msg := "disk failing"
	stub := &StubClient{
		GetSystemHealthWithResponseFunc: func(_ context.Context, _ ...systemapi.RequestEditorFn) (*systemapi.GetSystemHealthResponse, error) {
			return okHealthResp(systemapi.Health{
				Status:    systemapi.Healthy,
				CheckedAt: time.Now(),
				Components: []systemapi.ComponentHealth{
					{Name: "nas-1", Status: systemapi.Healthy},
					{Name: "unifi", Status: systemapi.Degraded, Message: &msg},
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
		GetSystemHealthWithResponseFunc: func(_ context.Context, _ ...systemapi.RequestEditorFn) (*systemapi.GetSystemHealthResponse, error) {
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
