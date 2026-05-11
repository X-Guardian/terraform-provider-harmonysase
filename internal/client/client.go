// SPDX-License-Identifier: MPL-2.0

// Package client is a thin Go HTTP client for the Check Point Harmony SASE
// public API (v2.3). It handles API-key/JWT exchange, regional routing,
// async-operation polling, and typed request/response decoding.
package client

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/hashicorp/go-retryablehttp"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"golang.org/x/time/rate"
)

// Documented Harmony SASE limit: 500 requests per 5-minute window per source
// IP (https://support.perimeter81.com/docs/api-getting-started). The default
// pins us to that ceiling. The provider exposes both knobs as configurable
// attributes for callers who run multiple processes from the same IP and need
// headroom, or who are on an enhanced tier.
const (
	defaultRateLimitPerMinute = 100
	defaultRateLimitBurst     = 10
)

// Region selects a Harmony SASE deployment.
type Region string

const (
	RegionUS Region = "us"
	RegionEU Region = "eu"
	RegionAU Region = "au"
	RegionIN Region = "in"
)

// Hosts derives the auth and API base URLs for a region. The US region uses
// the legacy api.perimeter81.com hostname; all others use the
// api.{region}.sase.checkpoint.com pattern.
func (r Region) Hosts() (authBase, apiBase string, err error) {
	switch r {
	case RegionUS, "":
		return "https://api.perimeter81.com", "https://api.perimeter81.com/api/rest", nil
	case RegionEU:
		return "https://api.eu.sase.checkpoint.com", "https://api.eu.sase.checkpoint.com/api/rest", nil
	case RegionAU:
		return "https://api.au.sase.checkpoint.com", "https://api.au.sase.checkpoint.com/api/rest", nil
	case RegionIN:
		return "https://api.in.sase.checkpoint.com", "https://api.in.sase.checkpoint.com/api/rest", nil
	default:
		return "", "", fmt.Errorf("unsupported region %q (want one of: us, eu, au, in)", r)
	}
}

// Config configures a new Client.
type Config struct {
	APIKey    string
	Region    Region
	Endpoint  string // optional override for both auth and API hosts (testing/mocks)
	UserAgent string

	// RateLimitPerMinute caps outgoing requests; 0 means use defaultRateLimitPerMinute.
	RateLimitPerMinute int
	// RateLimitBurst is the token-bucket burst size; 0 means use defaultRateLimitBurst.
	RateLimitBurst int
}

// Client talks to the Harmony SASE API. It is safe for concurrent use.
type Client struct {
	apiKey    string
	authBase  string // host for /api/v1/auth/authorize
	apiBase   string // base for /v2.3/...
	userAgent string

	http    *retryablehttp.Client
	limiter *rate.Limiter

	tokenMu     sync.Mutex
	tokenValue  string
	tokenExpiry time.Time
}

// New constructs a Client. It does not perform any network calls; the first
// API call triggers token acquisition.
func New(cfg Config) (*Client, error) {
	if strings.TrimSpace(cfg.APIKey) == "" {
		return nil, errors.New("api_key is required")
	}

	authBase, apiBase, err := cfg.Region.Hosts()
	if err != nil {
		return nil, err
	}
	if cfg.Endpoint != "" {
		// Endpoint override replaces both hosts. Spec puts auth at
		// /api/v1/... and the REST API at /api/rest/..., so the override is
		// expected to be the bare host.
		authBase = strings.TrimRight(cfg.Endpoint, "/")
		apiBase = authBase + "/api/rest"
	}

	rc := retryablehttp.NewClient()
	rc.Logger = nil
	rc.RetryMax = 4
	rc.RetryWaitMin = 1 * time.Second
	rc.RetryWaitMax = 10 * time.Second
	// RequestLogHook fires before each *retry* attempt (not the first send).
	// retry==0 means "first retry" i.e. the second send overall, so we log
	// the URL and attempt counter so transient failures are visible.
	rc.RequestLogHook = func(_ retryablehttp.Logger, r *http.Request, retry int) {
		if retry == 0 {
			return
		}
		tflog.Debug(r.Context(), "retrying harmonysase request", map[string]any{
			"method":  r.Method,
			"url":     r.URL.String(),
			"attempt": retry + 1,
		})
	}

	ua := cfg.UserAgent
	if ua == "" {
		ua = "terraform-provider-harmonysase"
	}

	rpm := cfg.RateLimitPerMinute
	if rpm <= 0 {
		rpm = defaultRateLimitPerMinute
	}
	burst := cfg.RateLimitBurst
	if burst <= 0 {
		burst = defaultRateLimitBurst
	}

	return &Client{
		apiKey:    cfg.APIKey,
		authBase:  authBase,
		apiBase:   apiBase,
		userAgent: ua,
		http:      rc,
		limiter:   rate.NewLimiter(rate.Every(time.Minute/time.Duration(rpm)), burst),
	}, nil
}

// do performs an authenticated HTTP request against the API base. The body,
// when non-nil, is JSON-encoded. The response body is JSON-decoded into out
// if out is non-nil and the status is 2xx. Non-2xx responses are returned as
// *APIError.
func (c *Client) do(ctx context.Context, method, path string, body, out any) (*http.Response, error) {
	return c.doAt(ctx, method, c.apiBase+path, body, out)
}

// doAuth performs a request against the auth host, used for token exchange.
// It does not inject a bearer header.
func (c *Client) doAuth(ctx context.Context, method, path string, body, out any) (*http.Response, error) {
	return c.doAtRaw(ctx, method, c.authBase+path, body, out, false)
}

// doAt performs an authenticated request against an arbitrary URL, used for
// async status polling where the API hands us a fully-qualified statusUrl.
func (c *Client) doAt(ctx context.Context, method, fullURL string, body, out any) (*http.Response, error) {
	return c.doAtRaw(ctx, method, fullURL, body, out, true)
}

func (c *Client) doAtRaw(ctx context.Context, method, fullURL string, body, out any, withAuth bool) (*http.Response, error) {
	// Block before issuing the request when we're at the rate-limit ceiling.
	// Wait honours ctx cancellation and surfaces the error cleanly.
	if c.limiter != nil {
		if err := c.limiter.Wait(ctx); err != nil {
			return nil, fmt.Errorf("rate limiter: %w", err)
		}
	}

	var bodyBytes []byte
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("marshal request body: %w", err)
		}
		bodyBytes = b
	}

	req, err := retryablehttp.NewRequestWithContext(ctx, method, fullURL, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", c.userAgent)
	if bodyBytes != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if withAuth {
		token, err := c.token(ctx)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%s %s: %w", method, fullURL, err)
	}

	// Read the body once; we may need it for both success-decode and error.
	respBody, readErr := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if readErr != nil {
		return resp, fmt.Errorf("%s %s: read response: %w", method, fullURL, readErr)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		ae := &APIError{
			Status:  resp.StatusCode,
			Method:  method,
			URL:     fullURL,
			RawBody: respBody,
		}
		// Best-effort decode of the {id, message} envelope.
		var env struct {
			ID      string `json:"id"`
			Message string `json:"message"`
		}
		if json.Unmarshal(respBody, &env) == nil {
			ae.Code = env.ID
			ae.Message = env.Message
		}
		return resp, ae
	}

	if out != nil && len(respBody) > 0 {
		if err := json.Unmarshal(respBody, out); err != nil {
			return resp, fmt.Errorf("%s %s: decode response: %w", method, fullURL, err)
		}
	}
	return resp, nil
}
