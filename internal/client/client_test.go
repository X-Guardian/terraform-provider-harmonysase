// SPDX-License-Identifier: MPL-2.0

package client

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// newTestServer wires up a single httptest server that handles both auth and
// API endpoints, then returns a Client configured to use it via Endpoint
// override.
func newTestServer(t *testing.T, handler http.HandlerFunc) (*Client, *httptest.Server) {
	t.Helper()
	ts := httptest.NewServer(handler)
	t.Cleanup(ts.Close)

	c, err := New(Config{APIKey: "test-key", Endpoint: ts.URL})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	// Disable retries to keep tests fast and deterministic.
	c.http.RetryMax = 0
	return c, ts
}

// writeAuthResponse writes a stub auth/authorize response with a 30-minute
// token. Used by endpoint tests that don't care about auth specifics.
func writeAuthResponse(w http.ResponseWriter) {
	expiry := time.Now().Add(30 * time.Minute).UTC().Format(time.RFC3339)
	_, _ = fmt.Fprintf(w, `{"data":{"tokenType":"bearer","accessToken":"tok","accessTokenExpire":%q}}`, expiry)
}

func TestRegionHosts(t *testing.T) {
	cases := []struct {
		region   Region
		wantAuth string
		wantAPI  string
		wantErr  bool
	}{
		{RegionUS, "https://api.perimeter81.com", "https://api.perimeter81.com/api/rest", false},
		{"", "https://api.perimeter81.com", "https://api.perimeter81.com/api/rest", false},
		{RegionEU, "https://api.eu.sase.checkpoint.com", "https://api.eu.sase.checkpoint.com/api/rest", false},
		{RegionAU, "https://api.au.sase.checkpoint.com", "https://api.au.sase.checkpoint.com/api/rest", false},
		{RegionIN, "https://api.in.sase.checkpoint.com", "https://api.in.sase.checkpoint.com/api/rest", false},
		{"mars", "", "", true},
	}
	for _, tc := range cases {
		auth, api, err := tc.region.Hosts()
		if (err != nil) != tc.wantErr {
			t.Errorf("region %q: err=%v wantErr=%v", tc.region, err, tc.wantErr)
		}
		if auth != tc.wantAuth || api != tc.wantAPI {
			t.Errorf("region %q: got (%s, %s) want (%s, %s)", tc.region, auth, api, tc.wantAuth, tc.wantAPI)
		}
	}
}

func TestNew_RequiresAPIKey(t *testing.T) {
	if _, err := New(Config{}); err == nil {
		t.Fatal("expected error for empty api_key")
	}
	if _, err := New(Config{APIKey: "  "}); err == nil {
		t.Fatal("expected error for whitespace api_key")
	}
}

// TestTokenCachingAndRefresh verifies (a) the first API call exchanges the
// API key for a token, (b) subsequent calls reuse the cached token, and (c)
// when expiry is within the leeway, a refresh is triggered.
func TestTokenCachingAndRefresh(t *testing.T) {
	var authCalls int32
	c, _ := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/auth/authorize":
			atomic.AddInt32(&authCalls, 1)
			// Return a token that's already inside the refresh leeway window
			// on the second call so we can assert the refresh.
			expiry := time.Now().Add(20 * time.Minute).UTC().Format(time.RFC3339)
			if atomic.LoadInt32(&authCalls) >= 2 {
				expiry = time.Now().Add(20 * time.Minute).UTC().Format(time.RFC3339)
			}
			_, _ = fmt.Fprintf(w, `{"data":{"tokenType":"bearer","accessToken":"tok-%d","accessTokenExpire":%q}}`, atomic.LoadInt32(&authCalls), expiry)
		case "/api/rest/v2.3/networks/standard/n1":
			if got := r.Header.Get("Authorization"); !strings.HasPrefix(got, "Bearer tok-") {
				t.Errorf("expected bearer header, got %q", got)
			}
			_, _ = fmt.Fprint(w, `{"id":"n1","name":"x"}`)
		default:
			http.NotFound(w, r)
		}
	})

	ctx := context.Background()
	if _, err := c.GetNetwork(ctx, "n1"); err != nil {
		t.Fatalf("GetNetwork 1: %v", err)
	}
	if _, err := c.GetNetwork(ctx, "n1"); err != nil {
		t.Fatalf("GetNetwork 2: %v", err)
	}
	if got := atomic.LoadInt32(&authCalls); got != 1 {
		t.Errorf("expected 1 auth call (cached), got %d", got)
	}

	// Force expiry into the leeway window and confirm refresh happens.
	c.tokenMu.Lock()
	c.tokenExpiry = time.Now().Add(tokenRefreshLeeway / 2)
	c.tokenMu.Unlock()

	if _, err := c.GetNetwork(ctx, "n1"); err != nil {
		t.Fatalf("GetNetwork 3: %v", err)
	}
	if got := atomic.LoadInt32(&authCalls); got != 2 {
		t.Errorf("expected 2 auth calls after expiry, got %d", got)
	}
}

func TestAPIError_NotFoundConflict(t *testing.T) {
	c, _ := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/auth/authorize":
			expiry := time.Now().Add(30 * time.Minute).UTC().Format(time.RFC3339)
			_, _ = fmt.Fprintf(w, `{"data":{"tokenType":"bearer","accessToken":"tok","accessTokenExpire":%q}}`, expiry)
		case "/api/rest/v2.3/networks/standard/missing":
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"id":"NOT_FOUND","message":"network missing not found"}`))
		case "/api/rest/v2.3/networks/standard/locked":
			w.WriteHeader(http.StatusConflict)
			_, _ = w.Write([]byte(`{"id":"CONFLICT","message":"in use"}`))
		}
	})

	ctx := context.Background()
	_, err := c.GetNetwork(ctx, "missing")
	if !IsNotFound(err) {
		t.Errorf("expected IsNotFound, got %v", err)
	}
	_, err = c.GetNetwork(ctx, "locked")
	if !IsConflict(err) {
		t.Errorf("expected IsConflict, got %v", err)
	}
}

// TestPoller_HappyPathAndFail covers the two terminal states. Cancellation is
// covered by TestPoller_ContextCancel.
func TestPoller_HappyPathAndFail(t *testing.T) {
	var pollCount int32
	c, ts := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/auth/authorize":
			expiry := time.Now().Add(30 * time.Minute).UTC().Format(time.RFC3339)
			_, _ = fmt.Fprintf(w, `{"data":{"tokenType":"bearer","accessToken":"tok","accessTokenExpire":%q}}`, expiry)
		case "/api/rest/v2.3/networks/standard/status/ok":
			n := atomic.AddInt32(&pollCount, 1)
			if n < 2 {
				_, _ = fmt.Fprint(w, `{"completed":false}`)
			} else {
				_, _ = fmt.Fprint(w, `{"completed":true,"result":{"statusCode":201,"resource":"/v2.3/networks/standard/n1"}}`)
			}
		case "/api/rest/v2.3/networks/standard/status/bad":
			_, _ = fmt.Fprint(w, `{"completed":true,"result":{"statusCode":422,"reason":["region exhausted"]}}`)
		}
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	op := AsyncOperationResponse{
		StatusURL:    ts.URL + "/api/rest/v2.3/networks/standard/status/ok",
		SamplingTime: 1,
	}
	result, err := c.WaitForOperation(ctx, op)
	if err != nil {
		t.Fatalf("happy path: %v", err)
	}
	if got := result.ResourceID(); got != "n1" {
		t.Errorf("got resource id %q want n1", got)
	}

	op.StatusURL = ts.URL + "/api/rest/v2.3/networks/standard/status/bad"
	if _, err := c.WaitForOperation(ctx, op); err == nil || !strings.Contains(err.Error(), "region exhausted") {
		t.Errorf("expected failure with reason, got %v", err)
	}
}

// TestPoller_CompletedWithoutStatusField guards the terminal-state check. The payload signals completion with the
// boolean `completed` and carries no `status` field, so a poller looking for one never terminates.
func TestPoller_CompletedWithoutStatusField(t *testing.T) {
	var pollCount int32
	c, ts := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/auth/authorize" {
			writeAuthResponse(w)
			return
		}
		atomic.AddInt32(&pollCount, 1)
		// A finished operation: no `status` key anywhere in the payload.
		_, _ = fmt.Fprint(w, `{"completed":true,"result":{"statusCode":200}}`)
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	op := AsyncOperationResponse{
		StatusURL:    ts.URL + "/api/rest/v2.3/networks/status/Is4TrXRyL5",
		SamplingTime: 15,
	}
	if _, err := c.WaitForOperation(ctx, op); err != nil {
		t.Fatalf("expected immediate completion, got %v", err)
	}
	if got := atomic.LoadInt32(&pollCount); got != 1 {
		t.Errorf("expected to stop after 1 poll, polled %d times", got)
	}
}

// TestPoller_NoStatusURL covers endpoints documented as returning a bare
// result envelope with no statusUrl on their 202 (network delete, region
// add/remove, instance remove). There is nothing to poll, so this must be a
// no-op rather than an error.
func TestPoller_NoStatusURL(t *testing.T) {
	c, _ := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/auth/authorize" {
			writeAuthResponse(w)
			return
		}
		t.Errorf("unexpected request to %s: nothing should be polled", r.URL.Path)
	})

	result, err := c.WaitForOperation(context.Background(), AsyncOperationResponse{})
	if err != nil {
		t.Fatalf("expected no-op, got %v", err)
	}
	if result.ResourceID() != "" {
		t.Errorf("expected empty result, got %q", result.ResourceID())
	}
}

func TestOperationResult_ResourceID(t *testing.T) {
	cases := []struct {
		resource string
		want     string
	}{
		{"/v2.3/networks/standard/n1/tunnels/wireguard/EVfxqQjn8K", "EVfxqQjn8K"},
		{"/v2.3/networks/standard/n1/tunnels/wireguard/EVfxqQjn8K/", "EVfxqQjn8K"},
		{"EVfxqQjn8K", "EVfxqQjn8K"},
		{"", ""},
		{"/", ""},
	}
	for _, tc := range cases {
		if got := (OperationResult{Resource: tc.resource}).ResourceID(); got != tc.want {
			t.Errorf("ResourceID(%q) = %q, want %q", tc.resource, got, tc.want)
		}
	}
}

func TestPoller_ContextCancel(t *testing.T) {
	c, ts := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/auth/authorize" {
			expiry := time.Now().Add(30 * time.Minute).UTC().Format(time.RFC3339)
			_, _ = fmt.Fprintf(w, `{"data":{"tokenType":"bearer","accessToken":"tok","accessTokenExpire":%q}}`, expiry)
			return
		}
		_, _ = fmt.Fprint(w, `{"completed":false}`)
	})

	ctx, cancel := context.WithTimeout(context.Background(), 250*time.Millisecond)
	defer cancel()

	op := AsyncOperationResponse{
		StatusURL:    ts.URL + "/api/rest/v2.3/networks/standard/status/never",
		SamplingTime: 1,
	}
	if _, err := c.WaitForOperation(ctx, op); err == nil {
		t.Fatal("expected context error, got nil")
	}
}

func TestPoller_RelativeStatusURL(t *testing.T) {
	c, _ := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/auth/authorize":
			expiry := time.Now().Add(30 * time.Minute).UTC().Format(time.RFC3339)
			_, _ = fmt.Fprintf(w, `{"data":{"tokenType":"bearer","accessToken":"tok","accessTokenExpire":%q}}`, expiry)
		case "/api/rest/v2.3/networks/standard/status/abc":
			_, _ = fmt.Fprint(w, `{"completed":true}`)
		default:
			http.NotFound(w, r)
		}
	})

	op := AsyncOperationResponse{StatusURL: "/v2.3/networks/standard/status/abc", SamplingTime: 1}
	if _, err := c.WaitForOperation(context.Background(), op); err != nil {
		t.Fatalf("relative path poll: %v", err)
	}
}
