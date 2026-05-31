package cmdutil_test

import (
	"bytes"
	"net/http"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/bwilczynski/hlctl/internal/cli/cmdutil"
	"github.com/bwilczynski/hlctl/internal/cli/flags"
)

func fakeTemplates() fstest.MapFS {
	return fstest.MapFS{
		"greet.tmpl": &fstest.MapFile{Data: []byte("hello {{.Name}}\n")},
	}
}

type greet struct{ Name string }

func TestView_Render_table(t *testing.T) {
	t.Cleanup(func() { flags.OutputFormat = "" })
	flags.OutputFormat = "table"

	v := cmdutil.View{Templates: fakeTemplates(), Name: "greet.tmpl"}
	var buf bytes.Buffer
	if err := v.Render(&buf, http.StatusOK, []byte(`{"name":"world"}`), greet{Name: "world"}); err != nil {
		t.Fatalf("Render: %v", err)
	}
	if got := buf.String(); got != "hello world\n" {
		t.Errorf("unexpected output: %q", got)
	}
}

func TestView_Render_json(t *testing.T) {
	t.Cleanup(func() { flags.OutputFormat = "" })
	flags.OutputFormat = "json"

	v := cmdutil.View{Templates: fakeTemplates(), Name: "greet.tmpl"}
	var buf bytes.Buffer
	body := []byte(`{"name":"world"}`)
	if err := v.Render(&buf, http.StatusOK, body, greet{Name: "world"}); err != nil {
		t.Fatalf("Render: %v", err)
	}
	if buf.String() != string(body) {
		t.Errorf("expected raw body in json mode, got %q", buf.String())
	}
}

func TestView_Render_statusMismatch_returnsParseError(t *testing.T) {
	v := cmdutil.View{Templates: fakeTemplates(), Name: "greet.tmpl"}
	body := []byte(`{"title":"Not Found","detail":"missing"}`)
	err := v.Render(&bytes.Buffer{}, http.StatusNotFound, body, nil)
	if err == nil {
		t.Fatal("expected error for non-OK status")
	}
	if !strings.Contains(err.Error(), "Not Found") {
		t.Errorf("expected ParseError output, got %v", err)
	}
}

func TestView_Render_customStatus(t *testing.T) {
	t.Cleanup(func() { flags.OutputFormat = "" })
	flags.OutputFormat = "table"

	v := cmdutil.View{Templates: fakeTemplates(), Name: "greet.tmpl", Status: http.StatusCreated}
	var buf bytes.Buffer
	if err := v.Render(&buf, http.StatusCreated, []byte(`{"name":"new"}`), greet{Name: "new"}); err != nil {
		t.Fatalf("Render: %v", err)
	}
	if got := buf.String(); got != "hello new\n" {
		t.Errorf("unexpected output: %q", got)
	}

	// 200 should now be treated as a mismatch.
	err := v.Render(&bytes.Buffer{}, http.StatusOK, []byte(`{"title":"oops"}`), nil)
	if err == nil {
		t.Fatal("expected error when status differs from configured Status")
	}
}
