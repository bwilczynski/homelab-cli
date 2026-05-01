// internal/cli/system/stub.go
package system

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"

	gen "github.com/bwilczynski/hlctl/internal/system"
)

// StubClient is a SystemClient that delegates each method to a configurable
// function field. Use in tests to inject controlled responses.
// When a function field is nil the method panics — always set the field under test.
type StubClient struct {
	GetSystemHealthFunc       func(ctx context.Context, reqEditors ...gen.RequestEditorFn) (*http.Response, error)
	ListSystemInfoFunc        func(ctx context.Context, params *gen.ListSystemInfoParams, reqEditors ...gen.RequestEditorFn) (*http.Response, error)
	ListSystemUtilizationFunc func(ctx context.Context, params *gen.ListSystemUtilizationParams, reqEditors ...gen.RequestEditorFn) (*http.Response, error)
	ListSystemUpdatesFunc     func(ctx context.Context, params *gen.ListSystemUpdatesParams, reqEditors ...gen.RequestEditorFn) (*http.Response, error)
	GetSystemUpdateFunc       func(ctx context.Context, updateId string, reqEditors ...gen.RequestEditorFn) (*http.Response, error)
	CheckSystemUpdatesFunc    func(ctx context.Context, params *gen.CheckSystemUpdatesParams, reqEditors ...gen.RequestEditorFn) (*http.Response, error)
}

func (s *StubClient) GetSystemHealth(ctx context.Context, reqEditors ...gen.RequestEditorFn) (*http.Response, error) {
	return s.GetSystemHealthFunc(ctx, reqEditors...)
}

func (s *StubClient) ListSystemInfo(ctx context.Context, params *gen.ListSystemInfoParams, reqEditors ...gen.RequestEditorFn) (*http.Response, error) {
	return s.ListSystemInfoFunc(ctx, params, reqEditors...)
}

func (s *StubClient) ListSystemUtilization(ctx context.Context, params *gen.ListSystemUtilizationParams, reqEditors ...gen.RequestEditorFn) (*http.Response, error) {
	return s.ListSystemUtilizationFunc(ctx, params, reqEditors...)
}

func (s *StubClient) ListSystemUpdates(ctx context.Context, params *gen.ListSystemUpdatesParams, reqEditors ...gen.RequestEditorFn) (*http.Response, error) {
	return s.ListSystemUpdatesFunc(ctx, params, reqEditors...)
}

func (s *StubClient) GetSystemUpdate(ctx context.Context, updateId string, reqEditors ...gen.RequestEditorFn) (*http.Response, error) {
	return s.GetSystemUpdateFunc(ctx, updateId, reqEditors...)
}

func (s *StubClient) CheckSystemUpdates(ctx context.Context, params *gen.CheckSystemUpdatesParams, reqEditors ...gen.RequestEditorFn) (*http.Response, error) {
	return s.CheckSystemUpdatesFunc(ctx, params, reqEditors...)
}

// jsonResponse builds an *http.Response with a JSON body and the given status code.
func jsonResponse(status int, body any) *http.Response {
	b, _ := json.Marshal(body)
	return &http.Response{
		StatusCode: status,
		Body:       io.NopCloser(strings.NewReader(string(b))),
	}
}
