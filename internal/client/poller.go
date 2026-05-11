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

// asyncOperationStatus is the GET /status/{id} response.
type asyncOperationStatus struct {
	Status    string          `json:"status"` // pending | completed | failed
	Result    json.RawMessage `json:"result,omitempty"`
	Error     string          `json:"error,omitempty"`
	CreatedAt string          `json:"createdAt,omitempty"`
	UpdatedAt string          `json:"updatedAt,omitempty"`
}

const (
	defaultPollInterval = 3 * time.Second
	minPollInterval     = 1 * time.Second
	maxPollInterval     = 15 * time.Second
)

// WaitForOperation polls the given async operation until it reaches a terminal
// state (completed or failed) or ctx is cancelled. The op's StatusURL is the
// fully-qualified URL returned by the API; SamplingTime is interpreted as
// seconds and clamped to [minPollInterval, maxPollInterval].
//
// On success, the raw "result" payload (which varies by operation) is
// returned for callers that want to decode it.
func (c *Client) WaitForOperation(ctx context.Context, op AsyncOperationResponse) (json.RawMessage, error) {
	if op.StatusURL == "" {
		return nil, fmt.Errorf("async operation: empty statusUrl")
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
			return nil, fmt.Errorf("async operation: %w", ctx.Err())
		case <-timer.C:
		}
		attempt++

		var st asyncOperationStatus
		if _, err := c.doAt(ctx, "GET", statusURL, nil, &st); err != nil {
			return nil, fmt.Errorf("poll %s: %w", statusURL, err)
		}

		switch strings.ToLower(st.Status) {
		case "completed":
			tflog.Debug(ctx, "harmonysase async operation completed", map[string]any{
				"status_url": statusURL,
				"attempts":   attempt,
				"elapsed":    time.Since(start).String(),
			})
			return st.Result, nil
		case "failed":
			msg := st.Error
			if msg == "" {
				msg = "operation failed"
			}
			return nil, fmt.Errorf("async operation failed: %s", msg)
		case "pending", "":
			tflog.Trace(ctx, "harmonysase async operation still pending", map[string]any{
				"status_url": statusURL,
				"attempt":    attempt,
				"elapsed":    time.Since(start).String(),
			})
		default:
			return nil, fmt.Errorf("async operation: unknown status %q", st.Status)
		}

		timer.Reset(interval)
	}
}
