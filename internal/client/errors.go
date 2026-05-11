// SPDX-License-Identifier: MPL-2.0

package client

import (
	"errors"
	"fmt"
)

// APIError is the canonical error returned by the client for non-2xx HTTP
// responses. Callers can match on Status (e.g. 404 to drop state, 409 to
// translate) and use the decoded body fields when available.
type APIError struct {
	Status  int
	Method  string
	URL     string
	Code    string // "id" field in the API error envelope
	Message string // "message" field in the API error envelope
	RawBody []byte
}

func (e *APIError) Error() string {
	if e.Message != "" {
		return fmt.Sprintf("%s %s: %d %s: %s", e.Method, e.URL, e.Status, e.Code, e.Message)
	}
	return fmt.Sprintf("%s %s: %d (no message)", e.Method, e.URL, e.Status)
}

// IsNotFound reports whether err is an APIError with HTTP 404.
func IsNotFound(err error) bool {
	var ae *APIError
	if errors.As(err, &ae) {
		return ae.Status == 404
	}
	return false
}

// IsConflict reports whether err is an APIError with HTTP 409.
func IsConflict(err error) bool {
	var ae *APIError
	if errors.As(err, &ae) {
		return ae.Status == 409
	}
	return false
}
