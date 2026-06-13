// internal/cli/system/stub.go
package system

import (
	"context"

	systemapi "github.com/bwilczynski/hlctl/internal/api/system"
)

// StubClient is a SystemClient that delegates each method to a configurable
// function field. Use in tests to inject controlled responses.
// When a function field is nil the method panics — always set the field under test.
type StubClient struct {
	GetSystemHealthWithResponseFunc       func(ctx context.Context, reqEditors ...systemapi.RequestEditorFn) (*systemapi.GetSystemHealthResponse, error)
	ListSystemInfoWithResponseFunc        func(ctx context.Context, params *systemapi.ListSystemInfoParams, reqEditors ...systemapi.RequestEditorFn) (*systemapi.ListSystemInfoResponse, error)
	ListSystemUtilizationWithResponseFunc func(ctx context.Context, params *systemapi.ListSystemUtilizationParams, reqEditors ...systemapi.RequestEditorFn) (*systemapi.ListSystemUtilizationResponse, error)
	ListSystemUpdatesWithResponseFunc     func(ctx context.Context, params *systemapi.ListSystemUpdatesParams, reqEditors ...systemapi.RequestEditorFn) (*systemapi.ListSystemUpdatesResponse, error)
	GetSystemUpdateWithResponseFunc       func(ctx context.Context, updateId string, reqEditors ...systemapi.RequestEditorFn) (*systemapi.GetSystemUpdateResponse, error)
	CheckSystemUpdatesWithResponseFunc    func(ctx context.Context, params *systemapi.CheckSystemUpdatesParams, reqEditors ...systemapi.RequestEditorFn) (*systemapi.CheckSystemUpdatesResponse, error)
}

func (s *StubClient) GetSystemHealthWithResponse(ctx context.Context, reqEditors ...systemapi.RequestEditorFn) (*systemapi.GetSystemHealthResponse, error) {
	return s.GetSystemHealthWithResponseFunc(ctx, reqEditors...)
}

func (s *StubClient) ListSystemInfoWithResponse(ctx context.Context, params *systemapi.ListSystemInfoParams, reqEditors ...systemapi.RequestEditorFn) (*systemapi.ListSystemInfoResponse, error) {
	return s.ListSystemInfoWithResponseFunc(ctx, params, reqEditors...)
}

func (s *StubClient) ListSystemUtilizationWithResponse(ctx context.Context, params *systemapi.ListSystemUtilizationParams, reqEditors ...systemapi.RequestEditorFn) (*systemapi.ListSystemUtilizationResponse, error) {
	return s.ListSystemUtilizationWithResponseFunc(ctx, params, reqEditors...)
}

func (s *StubClient) ListSystemUpdatesWithResponse(ctx context.Context, params *systemapi.ListSystemUpdatesParams, reqEditors ...systemapi.RequestEditorFn) (*systemapi.ListSystemUpdatesResponse, error) {
	return s.ListSystemUpdatesWithResponseFunc(ctx, params, reqEditors...)
}

func (s *StubClient) GetSystemUpdateWithResponse(ctx context.Context, updateId string, reqEditors ...systemapi.RequestEditorFn) (*systemapi.GetSystemUpdateResponse, error) {
	return s.GetSystemUpdateWithResponseFunc(ctx, updateId, reqEditors...)
}

func (s *StubClient) CheckSystemUpdatesWithResponse(ctx context.Context, params *systemapi.CheckSystemUpdatesParams, reqEditors ...systemapi.RequestEditorFn) (*systemapi.CheckSystemUpdatesResponse, error) {
	return s.CheckSystemUpdatesWithResponseFunc(ctx, params, reqEditors...)
}
