package cmdutil_test

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/bwilczynski/hlctl/internal/cli/cmdutil"
	"github.com/spf13/cobra"
)

type fakeClient struct{ name string }

func TestInjectClient_setsContextValueOnLeafExecution(t *testing.T) {
	parent := &cobra.Command{Use: "parent"}
	cmdutil.InjectClient(parent, func() (*fakeClient, error) {
		return &fakeClient{name: "real"}, nil
	})

	var seen *fakeClient
	leaf := &cobra.Command{
		Use: "leaf",
		RunE: func(cmd *cobra.Command, _ []string) error {
			seen = cmdutil.Client[*fakeClient](cmd)
			return nil
		},
	}
	parent.AddCommand(leaf)

	parent.SetArgs([]string{"leaf"})
	if err := parent.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if seen == nil || seen.name != "real" {
		t.Errorf("expected injected client, got %+v", seen)
	}
}

func TestInjectClient_propagatesBuildError(t *testing.T) {
	parent := &cobra.Command{Use: "parent", SilenceUsage: true, SilenceErrors: true}
	cmdutil.InjectClient(parent, func() (*fakeClient, error) {
		return nil, errBoom
	})
	leaf := &cobra.Command{Use: "leaf", RunE: func(_ *cobra.Command, _ []string) error { return nil }}
	parent.AddCommand(leaf)

	parent.SetArgs([]string{"leaf"})
	err := parent.Execute()
	if err == nil || err.Error() != "boom" {
		t.Errorf("expected build error to propagate, got %v", err)
	}
}

func TestSetClient_seedsContextForLeafStandalone(t *testing.T) {
	var seen *fakeClient
	leaf := &cobra.Command{
		Use: "leaf",
		RunE: func(cmd *cobra.Command, _ []string) error {
			seen = cmdutil.Client[*fakeClient](cmd)
			return nil
		},
	}
	cmdutil.SetClient[*fakeClient](leaf, &fakeClient{name: "stub"})

	if err := leaf.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if seen == nil || seen.name != "stub" {
		t.Errorf("expected seeded stub, got %+v", seen)
	}
}

func TestSetClient_preservesExistingContextValues(t *testing.T) {
	type otherKey struct{}
	leaf := &cobra.Command{Use: "leaf"}
	ctx := context.WithValue(context.Background(), otherKey{}, "kept")
	leaf.SetContext(ctx)

	cmdutil.SetClient[*fakeClient](leaf, &fakeClient{name: "stub"})

	if got := leaf.Context().Value(otherKey{}); got != "kept" {
		t.Errorf("expected pre-existing context value preserved, got %v", got)
	}
	if cmdutil.Client[*fakeClient](leaf).name != "stub" {
		t.Error("expected client also seeded")
	}
}

func TestClient_panicsWithDiagnosticWhenMissing(t *testing.T) {
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic when no client is injected")
		}
		msg := fmt.Sprint(r)
		if !strings.Contains(msg, "no client injected") {
			t.Errorf("expected diagnostic message, got %q", msg)
		}
	}()
	cmd := &cobra.Command{Use: "test"}
	cmd.SetContext(context.Background())
	_ = cmdutil.Client[*fakeClient](cmd)
}

var errBoom = stringError("boom")

type stringError string

func (e stringError) Error() string { return string(e) }
