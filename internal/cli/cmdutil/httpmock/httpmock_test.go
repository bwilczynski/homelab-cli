package httpmock_test

import (
	"io"
	"net/http"
	"testing"

	"github.com/bwilczynski/hlctl/internal/cli/cmdutil/httpmock"
)

func TestRegistry_matchesAndCounts(t *testing.T) {
	reg := httpmock.NewRegistry()
	reg.Register(httpmock.REST("GET", "/docker/containers"), httpmock.JSONResponse(map[string]any{"items": []any{}}))

	client := &http.Client{Transport: reg}
	resp, err := client.Get("http://localhost/docker/containers")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
	reg.Verify(t) // should not fail
}

func TestRegistry_noMatch(t *testing.T) {
	reg := httpmock.NewRegistry()
	client := &http.Client{Transport: reg}
	_, err := client.Get("http://localhost/not/registered")
	if err == nil {
		t.Fatal("expected error for unmatched request")
	}
}

func TestREST_glob(t *testing.T) {
	reg := httpmock.NewRegistry()
	reg.Register(
		httpmock.REST("POST", "/docker/containers/*/start"),
		httpmock.StatusStringResponse(http.StatusNoContent, ""),
	)
	client := &http.Client{Transport: reg}
	resp, err := client.Post("http://localhost/docker/containers/nas-1.homeassistant/start", "", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		t.Errorf("expected 204, got %d", resp.StatusCode)
	}
}

func TestStatusJSONResponse(t *testing.T) {
	reg := httpmock.NewRegistry()
	reg.Register(httpmock.REST("GET", "/x"), httpmock.StatusJSONResponse(404, map[string]any{"title": "Not Found"}))
	client := &http.Client{Transport: reg}
	resp, err := client.Get("http://localhost/x")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 404 {
		t.Errorf("expected 404, got %d", resp.StatusCode)
	}
	b, _ := io.ReadAll(resp.Body)
	if string(b) != `{"title":"Not Found"}` {
		t.Errorf("unexpected body: %s", b)
	}
}
