package apiclient

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

type problem struct {
	Title  string  `json:"title"`
	Detail *string `json:"detail,omitempty"`
}

// ParseError reads an RFC 9457 Problem Details body from resp and returns
// a user-friendly error. Call this on any non-2xx response.
func ParseError(resp *http.Response) error {
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("unexpected status %d", resp.StatusCode)
	}

	var p problem
	if err := json.Unmarshal(body, &p); err != nil || p.Title == "" {
		return fmt.Errorf("unexpected status %d", resp.StatusCode)
	}

	if p.Detail != nil && *p.Detail != "" {
		return fmt.Errorf("%s — %s", p.Title, *p.Detail)
	}
	return fmt.Errorf("%s", p.Title)
}
