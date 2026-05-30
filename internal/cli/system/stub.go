// internal/cli/system/stub.go
package system

import (
	"context"

	gen "github.com/bwilczynski/hlctl/internal/system"
)

// StubClient is a SystemClient that delegates each method to a configurable
// function field. Use in tests to inject controlled responses.
// When a function field is nil the method panics — always set the field under test.
type StubClient struct {
	GetSystemHealthWithResponseFunc       func(ctx context.Context, reqEditors ...gen.RequestEditorFn) (*gen.GetSystemHealthResponse, error)
	ListSystemInfoWithResponseFunc        func(ctx context.Context, params *gen.ListSystemInfoParams, reqEditors ...gen.RequestEditorFn) (*gen.ListSystemInfoResponse, error)
	ListSystemUtilizationWithResponseFunc func(ctx context.Context, params *gen.ListSystemUtilizationParams, reqEditors ...gen.RequestEditorFn) (*gen.ListSystemUtilizationResponse, error)
	ListSystemUpdatesWithResponseFunc     func(ctx context.Context, params *gen.ListSystemUpdatesParams, reqEditors ...gen.RequestEditorFn) (*gen.ListSystemUpdatesResponse, error)
	GetSystemUpdateWithResponseFunc       func(ctx context.Context, updateId string, reqEditors ...gen.RequestEditorFn) (*gen.GetSystemUpdateResponse, error)
	CheckSystemUpdatesWithResponseFunc    func(ctx context.Context, params *gen.CheckSystemUpdatesParams, reqEditors ...gen.RequestEditorFn) (*gen.CheckSystemUpdatesResponse, error)
}

func (s *StubClient) GetSystemHealthWithResponse(ctx context.Context, reqEditors ...gen.RequestEditorFn) (*gen.GetSystemHealthResponse, error) {
	return s.GetSystemHealthWithResponseFunc(ctx, reqEditors...)
}

func (s *StubClient) ListSystemInfoWithResponse(ctx context.Context, params *gen.ListSystemInfoParams, reqEditors ...gen.RequestEditorFn) (*gen.ListSystemInfoResponse, error) {
	return s.ListSystemInfoWithResponseFunc(ctx, params, reqEditors...)
}

func (s *StubClient) ListSystemUtilizationWithResponse(ctx context.Context, params *gen.ListSystemUtilizationParams, reqEditors ...gen.RequestEditorFn) (*gen.ListSystemUtilizationResponse, error) {
	return s.ListSystemUtilizationWithResponseFunc(ctx, params, reqEditors...)
}

func (s *StubClient) ListSystemUpdatesWithResponse(ctx context.Context, params *gen.ListSystemUpdatesParams, reqEditors ...gen.RequestEditorFn) (*gen.ListSystemUpdatesResponse, error) {
	return s.ListSystemUpdatesWithResponseFunc(ctx, params, reqEditors...)
}

func (s *StubClient) GetSystemUpdateWithResponse(ctx context.Context, updateId string, reqEditors ...gen.RequestEditorFn) (*gen.GetSystemUpdateResponse, error) {
	return s.GetSystemUpdateWithResponseFunc(ctx, updateId, reqEditors...)
}

func (s *StubClient) CheckSystemUpdatesWithResponse(ctx context.Context, params *gen.CheckSystemUpdatesParams, reqEditors ...gen.RequestEditorFn) (*gen.CheckSystemUpdatesResponse, error) {
	return s.CheckSystemUpdatesWithResponseFunc(ctx, params, reqEditors...)
}
