// SPDX-License-Identifier: MPL-2.0

package client

import (
	"encoding/json"
	"testing"
	"time"
)

func TestParseExpiry(t *testing.T) {
	now := time.Now()

	cases := []struct {
		name string
		raw  string
		want func(t *testing.T, got time.Time)
	}{
		{
			name: "rfc3339 string",
			raw:  `"2026-04-22T11:33:15Z"`,
			want: func(t *testing.T, got time.Time) {
				want, _ := time.Parse(time.RFC3339, "2026-04-22T11:33:15Z")
				if !got.Equal(want) {
					t.Errorf("got %v want %v", got, want)
				}
			},
		},
		{
			name: "unix seconds",
			raw:  `1809771195`, // = 2027-04-22T11:33:15Z
			want: func(t *testing.T, got time.Time) {
				if got.Year() != 2027 || got.Month() != 5 {
					t.Errorf("got %v, expected May 2027", got)
				}
			},
		},
		{
			name: "unix milliseconds",
			raw:  `1809771195000`,
			want: func(t *testing.T, got time.Time) {
				if got.Year() != 2027 || got.Month() != 5 {
					t.Errorf("got %v, expected May 2027", got)
				}
			},
		},
		{
			name: "string-encoded number",
			raw:  `"1809771195"`,
			want: func(t *testing.T, got time.Time) {
				if got.Year() != 2027 {
					t.Errorf("got %v, expected 2027", got)
				}
			},
		},
		{
			name: "empty",
			raw:  ``,
			want: func(t *testing.T, got time.Time) {
				// fallback = ~45 min from now
				if d := got.Sub(now); d < 40*time.Minute || d > 50*time.Minute {
					t.Errorf("expected ~45 min fallback, got %v from now (%v)", d, got)
				}
			},
		},
		{
			name: "garbage string",
			raw:  `"not-a-date"`,
			want: func(t *testing.T, got time.Time) {
				if d := got.Sub(now); d < 40*time.Minute || d > 50*time.Minute {
					t.Errorf("expected ~45 min fallback, got %v from now", d)
				}
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := parseExpiry(json.RawMessage(tc.raw))
			tc.want(t, got)
		})
	}
}
