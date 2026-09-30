package auth

import (
	"errors"
	"net/http"
	"strings"
	"testing"

	"golang.org/x/oauth2"
)

type errTokenSource struct{ err error }

func (s errTokenSource) Token() (*oauth2.Token, error) { return nil, s.err }

func retrieveError(status int, code, desc string) error {
	return &oauth2.RetrieveError{
		Response:         &http.Response{StatusCode: status},
		ErrorCode:        code,
		ErrorDescription: desc,
	}
}

func TestDiskSavingTokenSource_refreshErrors(t *testing.T) {
	tests := []struct {
		name        string
		err         error
		wantSession bool
	}{
		{
			name:        "invalid_grant",
			err:         retrieveError(http.StatusBadRequest, "invalid_grant", "Refresh token is invalid or has already been claimed by another client."),
			wantSession: true,
		},
		{
			// Dex returns invalid_request when the refresh token was replaced
			// by a newer login for the same user and client.
			name:        "dex replaced token",
			err:         retrieveError(http.StatusBadRequest, "invalid_request", "Refresh token is invalid or has already been claimed by another client."),
			wantSession: true,
		},
		{
			name:        "dex expired token",
			err:         retrieveError(http.StatusBadRequest, "invalid_request", "Refresh token expired."),
			wantSession: true,
		},
		{
			name:        "server error",
			err:         retrieveError(http.StatusInternalServerError, "invalid_request", ""),
			wantSession: false,
		},
		{
			name:        "network error",
			err:         errors.New("dial tcp: connection refused"),
			wantSession: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &diskSavingTokenSource{src: errTokenSource{err: tt.err}}
			_, err := s.Token()
			if err == nil {
				t.Fatal("expected error")
			}
			gotSession := strings.Contains(err.Error(), "hlctl auth login")
			if gotSession != tt.wantSession {
				t.Errorf("got %q, want session-expired message: %v", err, tt.wantSession)
			}
		})
	}
}
