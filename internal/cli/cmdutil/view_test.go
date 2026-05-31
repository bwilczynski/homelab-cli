package cmdutil_test

import (
	"bytes"
	"errors"
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

func TestView_RenderWith_tableInvokesFn(t *testing.T) {
	t.Cleanup(func() { flags.OutputFormat = "" })
	flags.OutputFormat = "table"

	called := 0
	v := cmdutil.View{Templates: fakeTemplates(), Name: "greet.tmpl"}
	var buf bytes.Buffer
	err := v.RenderWith(&buf, http.StatusOK, []byte(`{"name":"world"}`), func() (any, error) {
		called++
		return greet{Name: "world"}, nil
	})
	if err != nil {
		t.Fatalf("RenderWith: %v", err)
	}
	if called != 1 {
		t.Errorf("expected fn called once, got %d", called)
	}
	if got := buf.String(); got != "hello world\n" {
		t.Errorf("unexpected output: %q", got)
	}
}

func TestView_RenderWith_jsonSkipsFn(t *testing.T) {
	t.Cleanup(func() { flags.OutputFormat = "" })
	flags.OutputFormat = "json"

	called := 0
	v := cmdutil.View{Templates: fakeTemplates(), Name: "greet.tmpl"}
	var buf bytes.Buffer
	body := []byte(`{"name":"world"}`)
	err := v.RenderWith(&buf, http.StatusOK, body, func() (any, error) {
		called++
		return nil, nil
	})
	if err != nil {
		t.Fatalf("RenderWith: %v", err)
	}
	if called != 0 {
		t.Errorf("expected fn NOT called in json mode, got %d invocations", called)
	}
	if buf.String() != string(body) {
		t.Errorf("expected raw body, got %q", buf.String())
	}
}

func TestView_RenderWith_statusMismatchSkipsFn(t *testing.T) {
	called := 0
	v := cmdutil.View{Templates: fakeTemplates(), Name: "greet.tmpl"}
	err := v.RenderWith(&bytes.Buffer{}, http.StatusNotFound, []byte(`{"title":"Not Found"}`), func() (any, error) {
		called++
		return nil, nil
	})
	if err == nil {
		t.Fatal("expected error for non-OK status")
	}
	if called != 0 {
		t.Errorf("expected fn NOT called on status mismatch, got %d invocations", called)
	}
	if !strings.Contains(err.Error(), "Not Found") {
		t.Errorf("expected ParseError output, got %v", err)
	}
}

func TestView_RenderWith_fnErrorPropagates(t *testing.T) {
	t.Cleanup(func() { flags.OutputFormat = "" })
	flags.OutputFormat = "table"

	v := cmdutil.View{Templates: fakeTemplates(), Name: "greet.tmpl"}
	wantErr := errors.New("boom")
	err := v.RenderWith(&bytes.Buffer{}, http.StatusOK, []byte(`{}`), func() (any, error) {
		return nil, wantErr
	})
	if !errors.Is(err, wantErr) {
		t.Errorf("expected fn error to propagate, got %v", err)
	}
}
