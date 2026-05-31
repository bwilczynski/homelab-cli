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

type fakeUnion struct {
	Kind string
	Data string
}

func (u fakeUnion) Discriminator() (string, error) {
	if u.Kind == "" {
		return "", errors.New("missing discriminator")
	}
	return u.Kind, nil
}

func polyTemplates() fstest.MapFS {
	return fstest.MapFS{
		"a.tmpl": &fstest.MapFile{Data: []byte("A: {{.}}\n")},
		"b.tmpl": &fstest.MapFile{Data: []byte("B: {{.}}\n")},
	}
}

func newPolyView() cmdutil.PolymorphicView[fakeUnion] {
	return cmdutil.PolymorphicView[fakeUnion]{
		Templates: polyTemplates(),
		Variants: map[string]cmdutil.Variant[fakeUnion]{
			"a": {Template: "a.tmpl", Resolve: func(u fakeUnion) (any, error) { return u.Data, nil }},
			"b": {Template: "b.tmpl", Resolve: func(u fakeUnion) (any, error) { return u.Data, nil }},
		},
	}
}

func TestPolymorphicView_dispatchesToVariant(t *testing.T) {
	t.Cleanup(func() { flags.OutputFormat = "" })
	flags.OutputFormat = "table"

	v := newPolyView()

	var bufA bytes.Buffer
	if err := v.Render(&bufA, http.StatusOK, []byte(`{"kind":"a"}`), &fakeUnion{Kind: "a", Data: "alpha"}); err != nil {
		t.Fatalf("Render a: %v", err)
	}
	if got := bufA.String(); got != "A: alpha\n" {
		t.Errorf("variant a output: %q", got)
	}

	var bufB bytes.Buffer
	if err := v.Render(&bufB, http.StatusOK, []byte(`{"kind":"b"}`), &fakeUnion{Kind: "b", Data: "beta"}); err != nil {
		t.Fatalf("Render b: %v", err)
	}
	if got := bufB.String(); got != "B: beta\n" {
		t.Errorf("variant b output: %q", got)
	}
}

func TestPolymorphicView_jsonModeSkipsDiscriminator(t *testing.T) {
	t.Cleanup(func() { flags.OutputFormat = "" })
	flags.OutputFormat = "json"

	v := newPolyView()
	body := []byte(`{"kind":"a","data":"alpha"}`)
	var buf bytes.Buffer
	// Kind is empty — would error out of Discriminator() if called.
	if err := v.Render(&buf, http.StatusOK, body, &fakeUnion{}); err != nil {
		t.Fatalf("Render json: %v", err)
	}
	if buf.String() != string(body) {
		t.Errorf("expected raw body, got %q", buf.String())
	}
}

func TestPolymorphicView_statusMismatch(t *testing.T) {
	v := newPolyView()
	err := v.Render(&bytes.Buffer{}, http.StatusNotFound, []byte(`{"title":"Not Found"}`), &fakeUnion{Kind: "a"})
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "Not Found") {
		t.Errorf("expected ParseError output, got %v", err)
	}
}

func TestPolymorphicView_customStatus(t *testing.T) {
	t.Cleanup(func() { flags.OutputFormat = "" })
	flags.OutputFormat = "table"

	v := newPolyView()
	v.Status = http.StatusCreated

	var buf bytes.Buffer
	if err := v.Render(&buf, http.StatusCreated, []byte(`{}`), &fakeUnion{Kind: "a", Data: "alpha"}); err != nil {
		t.Fatalf("Render 201: %v", err)
	}
	if got := buf.String(); got != "A: alpha\n" {
		t.Errorf("custom status output: %q", got)
	}

	// 200 should now be treated as a mismatch.
	if err := v.Render(&bytes.Buffer{}, http.StatusOK, []byte(`{"title":"oops"}`), &fakeUnion{Kind: "a"}); err == nil {
		t.Fatal("expected mismatch error when status differs from configured Status")
	}
}

func TestPolymorphicView_unknownDiscriminator(t *testing.T) {
	t.Cleanup(func() { flags.OutputFormat = "" })
	flags.OutputFormat = "table"

	v := newPolyView()
	err := v.Render(&bytes.Buffer{}, http.StatusOK, []byte(`{}`), &fakeUnion{Kind: "c"})
	if err == nil {
		t.Fatal("expected error for unknown discriminator")
	}
	if !strings.Contains(err.Error(), "fakeUnion") || !strings.Contains(err.Error(), `"c"`) {
		t.Errorf("expected error to mention type and discriminator value, got %v", err)
	}
}

func TestPolymorphicView_nilDetail(t *testing.T) {
	t.Cleanup(func() { flags.OutputFormat = "" })
	flags.OutputFormat = "table"

	v := newPolyView()
	err := v.Render(&bytes.Buffer{}, http.StatusOK, []byte(`{}`), (*fakeUnion)(nil))
	if err == nil {
		t.Fatal("expected error for nil detail")
	}
}

func TestPolymorphicView_resolveError(t *testing.T) {
	t.Cleanup(func() { flags.OutputFormat = "" })
	flags.OutputFormat = "table"

	wantErr := errors.New("resolve boom")
	v := cmdutil.PolymorphicView[fakeUnion]{
		Templates: polyTemplates(),
		Variants: map[string]cmdutil.Variant[fakeUnion]{
			"a": {Template: "a.tmpl", Resolve: func(u fakeUnion) (any, error) { return nil, wantErr }},
		},
	}
	err := v.Render(&bytes.Buffer{}, http.StatusOK, []byte(`{}`), &fakeUnion{Kind: "a"})
	if !errors.Is(err, wantErr) {
		t.Errorf("expected resolve error to propagate, got %v", err)
	}
}

func TestPolymorphicView_discriminatorError(t *testing.T) {
	t.Cleanup(func() { flags.OutputFormat = "" })
	flags.OutputFormat = "table"

	v := newPolyView()
	err := v.Render(&bytes.Buffer{}, http.StatusOK, []byte(`{}`), &fakeUnion{Kind: ""})
	if err == nil {
		t.Fatal("expected error for empty discriminator")
	}
	if !strings.Contains(err.Error(), "missing discriminator") {
		t.Errorf("expected fakeUnion discriminator error to propagate, got %v", err)
	}
}
