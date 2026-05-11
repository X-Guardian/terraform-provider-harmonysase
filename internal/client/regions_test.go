// SPDX-License-Identifier: MPL-2.0

package client

import (
	"context"
	"fmt"
	"net/http"
	"testing"
)

func TestGetStandardRegions(t *testing.T) {
	c, _ := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/auth/authorize":
			writeAuthResponse(w)
		case "/api/rest/v2.3/networks/standard/harmony-sase-regions":
			if r.Method != http.MethodGet {
				t.Errorf("expected GET, got %s", r.Method)
			}
			_, _ = fmt.Fprint(w, `[{"id":"r-us","name":"us-east","displayName":"US East","countryCode":"US","continentCode":"NA"}]`)
		default:
			http.NotFound(w, r)
		}
	})

	got, err := c.GetStandardRegions(context.Background())
	if err != nil {
		t.Fatalf("GetStandardRegions: %v", err)
	}
	if len(got) != 1 || got[0].ID != "r-us" || got[0].CountryCode != "US" {
		t.Errorf("unexpected response: %+v", got)
	}
}

func TestGetEnhancedRegions(t *testing.T) {
	c, _ := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/auth/authorize":
			writeAuthResponse(w)
		case "/api/rest/v2.3/networks/enhanced/harmony-sase-regions":
			if r.Method != http.MethodGet {
				t.Errorf("expected GET, got %s", r.Method)
			}
			_, _ = fmt.Fprint(w, `[{"id":"r-eu","name":"eu-west","displayName":"EU West","countryCode":"IE","continentCode":"EU"}]`)
		default:
			http.NotFound(w, r)
		}
	})

	got, err := c.GetEnhancedRegions(context.Background())
	if err != nil {
		t.Fatalf("GetEnhancedRegions: %v", err)
	}
	if len(got) != 1 || got[0].ID != "r-eu" || got[0].ContinentCode != "EU" {
		t.Errorf("unexpected response: %+v", got)
	}
}
