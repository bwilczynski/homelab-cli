package apiclient

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
)

type problem struct {
	Title  string  `json:"title"`
	Detail *string `json:"detail,omitempty"`
}

// ParseError parses an RFC 9457 Problem Details body from already-read bytes and returns
// a user-friendly error. Call this on any non-2xx response using resp.StatusCode() and resp.Body.
func ParseError(statusCode int, body []byte) error {
	var p problem
	if err := json.Unmarshal(body, &p); err != nil || p.Title == "" {
		return fmt.Errorf("unexpected status %d", statusCode)
	}
	if p.Detail != nil && *p.Detail != "" {
		return fmt.Errorf("%s — %s", p.Title, *p.Detail)
	}
	return errors.New(p.Title)
}

// ParseErrorResponse reads an RFC 9457 Problem Details body from resp and returns
// a user-friendly error. Deprecated: only use before WithResponse migration is complete.
func ParseErrorResponse(resp *http.Response) error {
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("unexpected status %d", resp.StatusCode)
	}
	return ParseError(resp.StatusCode, body)
}
