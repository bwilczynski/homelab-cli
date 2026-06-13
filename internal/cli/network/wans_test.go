package network

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/bwilczynski/hlctl/internal/cli/cmdutil"
	networkapi "github.com/bwilczynski/hlctl/internal/api/network"
)

func okWansResp(list networkapi.WanList) *networkapi.ListWansResponse {
	b, _ := json.Marshal(list)
	return &networkapi.ListWansResponse{HTTPResponse: &http.Response{StatusCode: http.StatusOK}, Body: b, JSON200: &list}
}

func errWansResp(status int, body map[string]any) *networkapi.ListWansResponse {
	b, _ := json.Marshal(body)
	return &networkapi.ListWansResponse{HTTPResponse: &http.Response{StatusCode: status}, Body: b}
}

func okWanResp(data map[string]any) *networkapi.GetWanResponse {
	b, _ := json.Marshal(data)
	var typed networkapi.WanDetail
	_ = json.Unmarshal(b, &typed)
	return &networkapi.GetWanResponse{HTTPResponse: &http.Response{StatusCode: http.StatusOK}, Body: b, JSON200: &typed}
}

func errWanResp(status int, body map[string]any) *networkapi.GetWanResponse {
	b, _ := json.Marshal(body)
	return &networkapi.GetWanResponse{HTTPResponse: &http.Response{StatusCode: status}, Body: b}
}

func TestListWansCmd_tableOutput(t *testing.T) {
	stub := &StubClient{
		ListWansWithResponseFunc: func(_ context.Context, _ ...networkapi.RequestEditorFn) (*networkapi.ListWansResponse, error) {
			return okWansResp(networkapi.WanList{
				Items: []networkapi.Wan{
					{Id: "unifi.wan1", Name: "WAN 1", IpAddress: "203.0.113.42", Uptime: 86400, Status: networkapi.WanStatusConnected},
					{Id: "unifi.wan2", Name: "WAN 2", IpAddress: "198.51.100.7", Uptime: 0, Status: networkapi.WanStatusFailover},
				},
			}), nil
		},
	}
	cmd := newListWansCmd()
	cmdutil.SetClient[NetworkClient](cmd, stub)
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := buf.String()
	for _, want := range []string{"unifi.wan1", "WAN 1", "203.0.113.42", "connected", "unifi.wan2", "failover"} {
		if !strings.Contains(out, want) {
			t.Errorf("expected %q in output, got:\n%s", want, out)
		}
	}
}

func TestListWansCmd_apiError(t *testing.T) {
	stub := &StubClient{
		ListWansWithResponseFunc: func(_ context.Context, _ ...networkapi.RequestEditorFn) (*networkapi.ListWansResponse, error) {
			return errWansResp(http.StatusUnauthorized, map[string]any{
				"type": "https://homelab.local/problems/unauthorized", "title": "Unauthorized",
				"status": 401, "detail": "Bearer token missing",
			}), nil
		},
	}
	cmd := newListWansCmd()
	cmdutil.SetClient[NetworkClient](cmd, stub)
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

func TestGetWanCmd_connected(t *testing.T) {
	stub := &StubClient{
		GetWanWithResponseFunc: func(_ context.Context, _ string, _ ...networkapi.RequestEditorFn) (*networkapi.GetWanResponse, error) {
			return okWanResp(map[string]any{
				"id": "unifi.wan1", "uri": "/network/wans/unifi.wan1",
				"name": "WAN 1", "ipAddress": "203.0.113.42",
				"uptime": 86400, "status": "connected",
				"dnsServers": []string{"1.1.1.1", "1.0.0.1"},
			}), nil
		},
	}
	cmd := newGetWanCmd()
	cmdutil.SetClient[NetworkClient](cmd, stub)
	cmd.SetArgs([]string{"unifi.wan1"})
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := buf.String()
	for _, want := range []string{"unifi.wan1", "WAN 1", "203.0.113.42", "connected", "1d", "1.1.1.1", "1.0.0.1"} {
		if !strings.Contains(out, want) {
			t.Errorf("expected %q in output, got:\n%s", want, out)
		}
	}
}

func TestGetWanCmd_notFound(t *testing.T) {
	stub := &StubClient{
		GetWanWithResponseFunc: func(_ context.Context, _ string, _ ...networkapi.RequestEditorFn) (*networkapi.GetWanResponse, error) {
			return errWanResp(http.StatusNotFound, map[string]any{
				"type": "https://homelab.local/problems/not-found", "title": "Not Found",
				"status": 404, "detail": "wan not found",
			}), nil
		},
	}
	cmd := newGetWanCmd()
	cmdutil.SetClient[NetworkClient](cmd, stub)
	cmd.SetArgs([]string{"unifi.nonexistent"})
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "Not Found") {
		t.Errorf("expected 'Not Found' in error, got: %v", err)
	}
}
