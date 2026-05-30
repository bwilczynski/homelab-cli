package apiclient_test

import (
	"testing"

	"github.com/bwilczynski/hlctl/internal/apiclient"
)

func TestParseError_withDetail(t *testing.T) {
	body := []byte(`{"type":"https://example.com/problem","title":"Not Found","status":404,"detail":"container 'nas-1.foo' does not exist"}`)
	err := apiclient.ParseError(404, body)
	want := "Not Found — container 'nas-1.foo' does not exist"
	if err == nil || err.Error() != want {
		t.Errorf("got %v, want %q", err, want)
	}
}

func TestParseError_withoutDetail(t *testing.T) {
	body := []byte(`{"type":"https://example.com/problem","title":"Unauthorized","status":401}`)
	err := apiclient.ParseError(401, body)
	want := "Unauthorized"
	if err == nil || err.Error() != want {
		t.Errorf("got %v, want %q", err, want)
	}
}

func TestParseError_invalidBody(t *testing.T) {
	err := apiclient.ParseError(500, []byte("not json"))
	want := "unexpected status 500"
	if err == nil || err.Error() != want {
		t.Errorf("got %v, want %q", err, want)
	}
}
