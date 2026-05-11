// SPDX-License-Identifier: MPL-2.0

package client

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"time"
)

// tokenRefreshLeeway is how long before expiry the cached token is considered
// stale and refreshed. 60-minute tokens minus 5 minutes leaves a comfortable
// window for in-flight requests.
const tokenRefreshLeeway = 5 * time.Minute

type authorizeRequest struct {
	GrantType string `json:"grantType"`
	APIKey    string `json:"apiKey"`
}

// authorizeResponse mirrors the auth endpoint's response. accessTokenExpire
// has been observed as both an RFC 3339 string and a Unix-epoch number in
// the wild, so we accept any JSON value and parse leniently in parseExpiry.
type authorizeResponse struct {
	Data struct {
		TokenType         string          `json:"tokenType"`
		AccessToken       string          `json:"accessToken"`
		AccessTokenExpire json.RawMessage `json:"accessTokenExpire"`
	} `json:"data"`
}

// token returns a valid bearer token, refreshing if the cached one is missing
// or about to expire. Concurrent callers wait on a shared mutex.
func (c *Client) token(ctx context.Context) (string, error) {
	c.tokenMu.Lock()
	defer c.tokenMu.Unlock()

	if c.tokenValue != "" && time.Until(c.tokenExpiry) > tokenRefreshLeeway {
		return c.tokenValue, nil
	}

	var resp authorizeResponse
	_, err := c.doAuth(ctx, "POST", "/api/v1/auth/authorize", authorizeRequest{
		GrantType: "api_key",
		APIKey:    c.apiKey,
	}, &resp)
	if err != nil {
		return "", fmt.Errorf("authorize: %w", err)
	}
	if resp.Data.AccessToken == "" {
		return "", errors.New("authorize: empty accessToken in response")
	}

	c.tokenValue = resp.Data.AccessToken
	c.tokenExpiry = parseExpiry(resp.Data.AccessTokenExpire)
	return c.tokenValue, nil
}

// parseExpiry interprets the accessTokenExpire field. The API has been
// observed returning:
//   - an RFC 3339 string ("2026-04-22T11:33:15Z")
//   - a Unix-epoch number in seconds (1809771195)
//   - a Unix-epoch number in milliseconds (1809771195000)
//
// On any decode failure we fall back to a 45-minute TTL, well within the
// documented 60-minute token lifetime.
func parseExpiry(raw json.RawMessage) time.Time {
	fallback := time.Now().Add(45 * time.Minute)
	if len(raw) == 0 {
		return fallback
	}

	// Try string first (with surrounding quotes).
	var asString string
	if json.Unmarshal(raw, &asString) == nil && asString != "" {
		if t, err := time.Parse(time.RFC3339, asString); err == nil {
			return t
		}
		// String that's actually a number — fall through to numeric parse.
		if n, err := strconv.ParseInt(asString, 10, 64); err == nil {
			return epochToTime(n)
		}
		return fallback
	}

	// Try number.
	var asNum json.Number
	if err := json.Unmarshal(raw, &asNum); err == nil {
		if n, err := asNum.Int64(); err == nil {
			return epochToTime(n)
		}
	}

	return fallback
}

// epochToTime converts an integer that may be Unix seconds or milliseconds
// into a time.Time. We disambiguate by magnitude: any value past ~year 5000
// in seconds (roughly 10^11) is implausible, so we treat it as milliseconds.
func epochToTime(n int64) time.Time {
	const secondsCutoff = int64(1e11) // ~Nov 2286 in seconds
	if n > secondsCutoff {
		return time.Unix(n/1000, (n%1000)*int64(time.Millisecond))
	}
	return time.Unix(n, 0)
}
