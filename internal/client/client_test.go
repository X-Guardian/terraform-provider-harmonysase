// SPDX-License-Identifier: MPL-2.0

package client

import (
	"context"
	"encoding/json"
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
				_, _ = fmt.Fprint(w, `{"status":"pending"}`)
			} else {
				_, _ = fmt.Fprint(w, `{"status":"completed","result":{"id":"n1"}}`)
			}
		case "/api/rest/v2.3/networks/standard/status/bad":
			_, _ = fmt.Fprint(w, `{"status":"failed","error":"region exhausted"}`)
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
	var got struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(result, &got); err != nil {
		t.Fatalf("decode result: %v", err)
	}
	if got.ID != "n1" {
		t.Errorf("got id %q want n1", got.ID)
	}

	op.StatusURL = ts.URL + "/api/rest/v2.3/networks/standard/status/bad"
	if _, err := c.WaitForOperation(ctx, op); err == nil || !strings.Contains(err.Error(), "region exhausted") {
		t.Errorf("expected failure with reason, got %v", err)
	}
}

func TestPoller_ContextCancel(t *testing.T) {
	c, ts := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/auth/authorize" {
			expiry := time.Now().Add(30 * time.Minute).UTC().Format(time.RFC3339)
			_, _ = fmt.Fprintf(w, `{"data":{"tokenType":"bearer","accessToken":"tok","accessTokenExpire":%q}}`, expiry)
			return
		}
		_, _ = fmt.Fprint(w, `{"status":"pending"}`)
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
			_, _ = fmt.Fprint(w, `{"status":"completed"}`)
		default:
			http.NotFound(w, r)
		}
	})

	op := AsyncOperationResponse{StatusURL: "/v2.3/networks/standard/status/abc", SamplingTime: 1}
	if _, err := c.WaitForOperation(context.Background(), op); err != nil {
		t.Fatalf("relative path poll: %v", err)
	}
}
