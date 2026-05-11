// SPDX-License-Identifier: MPL-2.0

package client

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"testing"
)

func TestCreateNetwork(t *testing.T) {
	c, _ := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/auth/authorize":
			writeAuthResponse(w)
		case "/api/rest/v2.3/networks/standard":
			if r.Method != http.MethodPost {
				t.Errorf("expected POST, got %s", r.Method)
			}
			body, _ := io.ReadAll(r.Body)
			var got DeployNetworkPayload
			if err := json.Unmarshal(body, &got); err != nil {
				t.Fatalf("decode body: %v", err)
			}
			if got.Network.Name != "n1" {
				t.Errorf("expected network.name=n1, got %q", got.Network.Name)
			}
			if len(got.Regions) != 1 || got.Regions[0].HarmonySaseRegionID != "r-1" {
				t.Errorf("unexpected regions: %+v", got.Regions)
			}
			_, _ = fmt.Fprint(w, `{"statusUrl":"https://example.test/status/op-1","samplingTime":1}`)
		default:
			http.NotFound(w, r)
		}
	})

	payload := DeployNetworkPayload{
		Regions: []CreateRegionInNetworkPayload{{HarmonySaseRegionID: "r-1"}},
	}
	payload.Network.Name = "n1"

	op, err := c.CreateNetwork(context.Background(), payload)
	if err != nil {
		t.Fatalf("CreateNetwork: %v", err)
	}
	if op.StatusURL == "" {
		t.Errorf("expected non-empty StatusURL, got %+v", op)
	}
}

func TestUpdateNetwork(t *testing.T) {
	c, _ := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/auth/authorize":
			writeAuthResponse(w)
		case "/api/rest/v2.3/networks/standard/n1":
			if r.Method != http.MethodPut {
				t.Errorf("expected PUT, got %s", r.Method)
			}
			body, _ := io.ReadAll(r.Body)
			var got UpdateNetworkPayload
			if err := json.Unmarshal(body, &got); err != nil {
				t.Fatalf("decode body: %v", err)
			}
			if got.Network.Name != "renamed" {
				t.Errorf("expected network.name=renamed, got %q", got.Network.Name)
			}
			_, _ = fmt.Fprint(w, `{"id":"n1","name":"renamed","subnet":"10.0.0.0/16"}`)
		default:
			http.NotFound(w, r)
		}
	})

	var payload UpdateNetworkPayload
	payload.Network.Name = "renamed"

	got, err := c.UpdateNetwork(context.Background(), "n1", payload)
	if err != nil {
		t.Fatalf("UpdateNetwork: %v", err)
	}
	if got.ID != "n1" || got.Name != "renamed" {
		t.Errorf("unexpected response: %+v", got)
	}
}

func TestDeleteNetwork(t *testing.T) {
	c, _ := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/auth/authorize":
			writeAuthResponse(w)
		case "/api/rest/v2.3/networks/standard/n1":
			if r.Method != http.MethodDelete {
				t.Errorf("expected DELETE, got %s", r.Method)
			}
			_, _ = fmt.Fprint(w, `{"statusUrl":"https://example.test/status/del","samplingTime":1}`)
		default:
			http.NotFound(w, r)
		}
	})

	op, err := c.DeleteNetwork(context.Background(), "n1")
	if err != nil {
		t.Fatalf("DeleteNetwork: %v", err)
	}
	if op.StatusURL == "" {
		t.Errorf("expected non-empty StatusURL, got %+v", op)
	}
}

func TestAddRegionsToNetwork(t *testing.T) {
	c, _ := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/auth/authorize":
			writeAuthResponse(w)
		case "/api/rest/v2.3/networks/standard/n1/regions":
			if r.Method != http.MethodPut {
				t.Errorf("expected PUT, got %s", r.Method)
			}
			body, _ := io.ReadAll(r.Body)
			var got []CreateRegionInNetworkPayload
			if err := json.Unmarshal(body, &got); err != nil {
				t.Fatalf("decode body: %v", err)
			}
			if len(got) != 1 || got[0].HarmonySaseRegionID != "r-2" {
				t.Errorf("unexpected regions: %+v", got)
			}
			_, _ = fmt.Fprint(w, `{"statusUrl":"https://example.test/status/add","samplingTime":1}`)
		default:
			http.NotFound(w, r)
		}
	})

	op, err := c.AddRegionsToNetwork(context.Background(), "n1", []CreateRegionInNetworkPayload{
		{HarmonySaseRegionID: "r-2"},
	})
	if err != nil {
		t.Fatalf("AddRegionsToNetwork: %v", err)
	}
	if op.StatusURL == "" {
		t.Errorf("expected non-empty StatusURL, got %+v", op)
	}
}

func TestRemoveRegionsFromNetwork(t *testing.T) {
	c, _ := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/auth/authorize":
			writeAuthResponse(w)
		case "/api/rest/v2.3/networks/standard/n1/regions":
			if r.Method != http.MethodDelete {
				t.Errorf("expected DELETE, got %s", r.Method)
			}
			body, _ := io.ReadAll(r.Body)
			var got RemoveRegionsPayload
			if err := json.Unmarshal(body, &got); err != nil {
				t.Fatalf("decode body: %v", err)
			}
			if len(got.RegionIDs) != 2 || got.RegionIDs[0] != "reg-a" || got.RegionIDs[1] != "reg-b" {
				t.Errorf("unexpected regionIds: %+v", got.RegionIDs)
			}
			_, _ = fmt.Fprint(w, `{"statusUrl":"https://example.test/status/rm","samplingTime":1}`)
		default:
			http.NotFound(w, r)
		}
	})

	op, err := c.RemoveRegionsFromNetwork(context.Background(), "n1", []string{"reg-a", "reg-b"})
	if err != nil {
		t.Fatalf("RemoveRegionsFromNetwork: %v", err)
	}
	if op.StatusURL == "" {
		t.Errorf("expected non-empty StatusURL, got %+v", op)
	}
}

func TestGetRegion(t *testing.T) {
	c, _ := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/auth/authorize":
			writeAuthResponse(w)
		case "/api/rest/v2.3/networks/standard/n1/regions/reg-a":
			if r.Method != http.MethodGet {
				t.Errorf("expected GET, got %s", r.Method)
			}
			_, _ = fmt.Fprint(w, `{"id":"reg-a","network":"n1","name":"us-east-1","dns":"1.2.3.4"}`)
		default:
			http.NotFound(w, r)
		}
	})

	got, err := c.GetRegion(context.Background(), "n1", "reg-a")
	if err != nil {
		t.Fatalf("GetRegion: %v", err)
	}
	if got.ID != "reg-a" || got.Name != "us-east-1" {
		t.Errorf("unexpected response: %+v", got)
	}
}
