package network

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/bwilczynski/hlctl/internal/cli/cmdutil"
	gen "github.com/bwilczynski/hlctl/internal/network"
)

func okSsidsResp(list gen.SsidList) *gen.ListSsidsResponse {
	b, _ := json.Marshal(list)
	return &gen.ListSsidsResponse{HTTPResponse: &http.Response{StatusCode: http.StatusOK}, Body: b, JSON200: &list}
}

func errSsidsResp(status int, body map[string]any) *gen.ListSsidsResponse {
	b, _ := json.Marshal(body)
	return &gen.ListSsidsResponse{HTTPResponse: &http.Response{StatusCode: status}, Body: b}
}

func okSsidResp(data map[string]any) *gen.GetSsidResponse {
	b, _ := json.Marshal(data)
	var typed gen.SsidDetail
	_ = json.Unmarshal(b, &typed)
	return &gen.GetSsidResponse{HTTPResponse: &http.Response{StatusCode: http.StatusOK}, Body: b, JSON200: &typed}
}

func errSsidResp(status int, body map[string]any) *gen.GetSsidResponse {
	b, _ := json.Marshal(body)
	return &gen.GetSsidResponse{HTTPResponse: &http.Response{StatusCode: status}, Body: b}
}

func TestListSsidsCmd_tableOutput(t *testing.T) {
	stub := &StubClient{
		ListSsidsWithResponseFunc: func(_ context.Context, _ ...gen.RequestEditorFn) (*gen.ListSsidsResponse, error) {
			return okSsidsResp(gen.SsidList{
				Items: []gen.Ssid{
					{
						Id: "unifi.home", Name: "Home", VlanId: 1,
						Bands:      []gen.WifiBand{gen.WifiBandBand2g, gen.WifiBandBand5g, gen.WifiBandBand6g},
						NumClients: 12,
					},
					{
						Id: "unifi.iot", Name: "IoT", VlanId: 20,
						Bands:      []gen.WifiBand{gen.WifiBandBand2g, gen.WifiBandBand5g},
						NumClients: 8,
					},
				},
			}), nil
		},
	}
	cmd := newListSsidsCmd()
	cmdutil.SetClient[NetworkClient](cmd, stub)
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := buf.String()
	for _, want := range []string{"unifi.home", "Home", "unifi.iot", "IoT", "2.4 GHz", "5 GHz", "6 GHz", "12", "8"} {
		if !strings.Contains(out, want) {
			t.Errorf("expected %q in output, got:\n%s", want, out)
		}
	}
}

func TestListSsidsCmd_apiError(t *testing.T) {
	stub := &StubClient{
		ListSsidsWithResponseFunc: func(_ context.Context, _ ...gen.RequestEditorFn) (*gen.ListSsidsResponse, error) {
			return errSsidsResp(http.StatusUnauthorized, map[string]any{
				"type": "https://homelab.local/problems/unauthorized", "title": "Unauthorized",
				"status": 401, "detail": "Bearer token missing",
			}), nil
		},
	}
	cmd := newListSsidsCmd()
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

func TestGetSsidCmd_withClients(t *testing.T) {
	stub := &StubClient{
		GetSsidWithResponseFunc: func(_ context.Context, _ string, _ ...gen.RequestEditorFn) (*gen.GetSsidResponse, error) {
			return okSsidResp(map[string]any{
				"id": "unifi.iot", "uri": "/network/ssids/unifi.iot",
				"name": "IoT", "vlanId": 20,
				"bands":      []string{"band2g", "band5g"},
				"numClients": 2,
				"securityProtocol": "wpa2",
				"clients": []map[string]any{
					{"kind": "client", "id": "unifi.sonos", "uri": "/network/clients/unifi.sonos", "name": "Sonos One SL"},
					{"kind": "client", "id": "unifi.hue", "uri": "/network/clients/unifi.hue", "name": "Philips Hue Bridge"},
				},
				"broadcastingAps": []map[string]any{
					{"kind": "device", "id": "unifi.ap-lr", "uri": "/network/devices/unifi.ap-lr", "name": "AP Living Room"},
				},
			}), nil
		},
	}
	cmd := newGetSsidCmd()
	cmdutil.SetClient[NetworkClient](cmd, stub)
	cmd.SetArgs([]string{"unifi.iot"})
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := buf.String()
	for _, want := range []string{"unifi.iot", "IoT", "20", "wpa2", "Sonos One SL", "Philips Hue Bridge", "AP Living Room", "CLIENTS", "BROADCASTING APs"} {
		if !strings.Contains(out, want) {
			t.Errorf("expected %q in output, got:\n%s", want, out)
		}
	}
}

func TestGetSsidCmd_notFound(t *testing.T) {
	stub := &StubClient{
		GetSsidWithResponseFunc: func(_ context.Context, _ string, _ ...gen.RequestEditorFn) (*gen.GetSsidResponse, error) {
			return errSsidResp(http.StatusNotFound, map[string]any{
				"type": "https://homelab.local/problems/not-found", "title": "Not Found",
				"status": 404, "detail": "ssid not found",
			}), nil
		},
	}
	cmd := newGetSsidCmd()
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
