package apiclient_test

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/bwilczynski/hlctl/internal/apiclient"
)

func problemResponse(status int, title, detail string) *http.Response {
	body := map[string]any{"type": "https://example.com/problem", "title": title, "status": status}
	if detail != "" {
		body["detail"] = detail
	}
	b, _ := json.Marshal(body)
	return &http.Response{
		StatusCode: status,
		Body:       io.NopCloser(strings.NewReader(string(b))),
	}
}

func TestParseError_withDetail(t *testing.T) {
	resp := problemResponse(404, "Not Found", "container 'nas-1.foo' does not exist")
	err := apiclient.ParseError(resp)
	want := "Not Found — container 'nas-1.foo' does not exist"
	if err == nil || err.Error() != want {
		t.Errorf("got %v, want %q", err, want)
	}
}

func TestParseError_withoutDetail(t *testing.T) {
	resp := problemResponse(401, "Unauthorized", "")
	err := apiclient.ParseError(resp)
	want := "Unauthorized"
	if err == nil || err.Error() != want {
		t.Errorf("got %v, want %q", err, want)
	}
}

func TestParseError_invalidBody(t *testing.T) {
	resp := &http.Response{
		StatusCode: 500,
		Body:       io.NopCloser(strings.NewReader("not json")),
	}
	err := apiclient.ParseError(resp)
	want := "unexpected status 500"
	if err == nil || err.Error() != want {
		t.Errorf("got %v, want %q", err, want)
	}
}
