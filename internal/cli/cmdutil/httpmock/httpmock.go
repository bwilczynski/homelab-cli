// Package httpmock provides a fake http.RoundTripper for testing commands
// that build real HTTP clients via their Options.HTTPClient field.
package httpmock

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"testing"
)

// Matcher returns true when a request should be handled by its paired Responder.
type Matcher func(*http.Request) bool

// Responder produces an HTTP response for a matched request.
type Responder func(*http.Request) (*http.Response, error)

type registered struct {
	matcher   Matcher
	responder Responder
	count     int
}

// Registry is an http.RoundTripper that matches requests against registered
// matchers and calls the associated responder. Unmatched requests return an error.
type Registry struct {
	mu   sync.Mutex
	regs []*registered
}

// NewRegistry returns an empty Registry.
func NewRegistry() *Registry { return &Registry{} }

// Register adds a matcher+responder pair. Matchers are checked in registration order.
func (r *Registry) Register(matcher Matcher, responder Responder) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.regs = append(r.regs, &registered{matcher: matcher, responder: responder})
}

// RoundTrip implements http.RoundTripper. Returns an error if no matcher matches.
func (r *Registry) RoundTrip(req *http.Request) (*http.Response, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, reg := range r.regs {
		if reg.matcher(req) {
			reg.count++
			return reg.responder(req)
		}
	}
	return nil, fmt.Errorf("httpmock: no match for %s %s", req.Method, req.URL.Path)
}

// Verify fails the test if any registered matcher was never matched.
func (r *Registry) Verify(t *testing.T) {
	t.Helper()
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, reg := range r.regs {
		if reg.count == 0 {
			t.Errorf("httpmock: registered handler was never called")
		}
	}
}

// REST returns a Matcher that checks method (exact) and path (glob: * matches [^/]+).
func REST(method, pathPattern string) Matcher {
	regexStr := "^" + strings.ReplaceAll(regexp.QuoteMeta(pathPattern), `\*`, `[^/]+`) + "$"
	pathRe := regexp.MustCompile(regexStr)
	return func(req *http.Request) bool {
		return req.Method == method && pathRe.MatchString(req.URL.Path)
	}
}

// JSONResponse returns a 200 OK with body marshalled as JSON.
func JSONResponse(body any) Responder {
	return func(_ *http.Request) (*http.Response, error) {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(bytes.NewReader(b)),
		}, nil
	}
}

// StatusStringResponse returns a response with the given status code and plain text body.
func StatusStringResponse(status int, body string) Responder {
	return func(_ *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: status,
			Header:     http.Header{},
			Body:       io.NopCloser(strings.NewReader(body)),
		}, nil
	}
}

// StatusJSONResponse returns a response with the given status code and body marshalled as JSON.
func StatusJSONResponse(status int, body any) Responder {
	return func(_ *http.Request) (*http.Response, error) {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		return &http.Response{
			StatusCode: status,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(bytes.NewReader(b)),
		}, nil
	}
}
