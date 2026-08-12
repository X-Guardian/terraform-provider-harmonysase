// SPDX-License-Identifier: MPL-2.0

package client

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/hashicorp/terraform-plugin-log/tflog"
)

// AsyncOperationResponse is the 202-Accepted envelope returned by every
// mutating endpoint on networks/regions/instances/tunnels.
type AsyncOperationResponse struct {
	StatusURL    string `json:"statusUrl"`
	SamplingTime int    `json:"samplingTime"`
}

// asyncOperationStatus is the GET /status/{id} response. The API signals terminal state with the boolean `completed`;
// success or failure of the underlying operation is carried by result.statusCode, with human-readable causes in
// result.reason.
type asyncOperationStatus struct {
	Completed bool                 `json:"completed"`
	Result    asyncOperationResult `json:"result,omitempty"`
}

// asyncOperationResult is the `result` envelope of asyncOperationStatus.
type asyncOperationResult struct {
	Resource   string          `json:"resource,omitempty"`
	StatusCode int             `json:"statusCode,omitempty"`
	Reason     []string        `json:"reason,omitempty"`
	raw        json.RawMessage `json:"-"`
}

// UnmarshalJSON keeps the raw payload alongside the decoded fields so callers that need operation-specific data
// (e.g. the created tunnel object) can decode it themselves.
func (r *asyncOperationResult) UnmarshalJSON(b []byte) error {
	type alias asyncOperationResult
	var a alias
	if err := json.Unmarshal(b, &a); err != nil {
		return err
	}
	*r = asyncOperationResult(a)
	r.raw = append(json.RawMessage(nil), b...)
	return nil
}

const (
	defaultPollInterval = 3 * time.Second
	minPollInterval     = 1 * time.Second
	maxPollInterval     = 15 * time.Second
)

// OperationResult is the outcome of a completed async operation.
type OperationResult struct {
	// Resource is the API path of the affected resource, if the operation created or modified one. Empty for
	// operations that address no resource.
	Resource string
	// Raw is the undecoded `result` payload, for callers needing more.
	Raw json.RawMessage
}

// ResourceID returns the trailing path segment of Resource, which for create operations is the ID of the newly
// created resource. Returns "" if Resource is empty.
func (r OperationResult) ResourceID() string {
	trimmed := strings.TrimRight(r.Resource, "/")
	if trimmed == "" {
		return ""
	}
	if i := strings.LastIndex(trimmed, "/"); i >= 0 {
		return trimmed[i+1:]
	}
	return trimmed
}

// WaitForOperation polls the given async operation until it reaches a terminal state (completed or failed) or ctx is
// cancelled. The op's StatusURL is the fully-qualified URL returned by the API; SamplingTime is interpreted as seconds
// and clamped to [minPollInterval, maxPollInterval].
//
// On success, the operation's result envelope is returned; for create operations its ResourceID is the new resource's
// ID.
//
// An empty StatusURL means the endpoint completed synchronously: there is nothing to poll, and a zero result is
// returned. Several endpoints are documented as returning a bare result envelope with no statusUrl on their 202, so
// this is not an error.
func (c *Client) WaitForOperation(ctx context.Context, op AsyncOperationResponse) (OperationResult, error) {
	if op.StatusURL == "" {
		tflog.Debug(ctx, "harmonysase operation returned no statusUrl; treating as already complete", nil)
		return OperationResult{}, nil
	}

	interval := time.Duration(op.SamplingTime) * time.Second
	switch {
	case interval <= 0:
		interval = defaultPollInterval
	case interval < minPollInterval:
		interval = minPollInterval
	case interval > maxPollInterval:
		interval = maxPollInterval
	}

	statusURL := op.StatusURL
	if !strings.HasPrefix(statusURL, "http://") && !strings.HasPrefix(statusURL, "https://") {
		// Some API responses return a relative path; resolve against apiBase.
		if !strings.HasPrefix(statusURL, "/") {
			statusURL = "/" + statusURL
		}
		statusURL = c.apiBase + statusURL
	}

	tflog.Debug(ctx, "polling harmonysase async operation", map[string]any{
		"status_url": statusURL,
		"interval":   interval.String(),
	})

	timer := time.NewTimer(0)
	defer timer.Stop()

	start := time.Now()
	attempt := 0
	for {
		select {
		case <-ctx.Done():
			return OperationResult{}, fmt.Errorf("async operation: %w", ctx.Err())
		case <-timer.C:
		}
		attempt++

		var st asyncOperationStatus
		if _, err := c.doAt(ctx, "GET", statusURL, nil, &st); err != nil {
			return OperationResult{}, fmt.Errorf("poll %s: %w", statusURL, err)
		}

		if st.Completed {
			tflog.Debug(ctx, "harmonysase async operation completed", map[string]any{
				"status_url":  statusURL,
				"attempts":    attempt,
				"elapsed":     time.Since(start).String(),
				"status_code": st.Result.StatusCode,
			})
			// A completed operation still reports its own success or failure via the HTTP-style status code in the
			// result envelope. Treat a zero code as success: some operations return an empty result body.
			if code := st.Result.StatusCode; code != 0 && (code < 200 || code > 299) {
				msg := strings.Join(st.Result.Reason, "; ")
				if msg == "" {
					msg = "operation failed"
				}
				return OperationResult{}, fmt.Errorf("async operation failed (status %d): %s", code, msg)
			}
			return OperationResult{Resource: st.Result.Resource, Raw: st.Result.raw}, nil
		}

		tflog.Trace(ctx, "harmonysase async operation still pending", map[string]any{
			"status_url": statusURL,
			"attempt":    attempt,
			"elapsed":    time.Since(start).String(),
		})

		timer.Reset(interval)
	}
}
