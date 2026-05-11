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

func TestAddInstances(t *testing.T) {
	c, _ := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/auth/authorize":
			writeAuthResponse(w)
		case "/api/rest/v2.3/networks/standard/n1/instances":
			if r.Method != http.MethodPost {
				t.Errorf("expected POST, got %s", r.Method)
			}
			body, _ := io.ReadAll(r.Body)
			var got CreateInstancesPayload
			if err := json.Unmarshal(body, &got); err != nil {
				t.Fatalf("decode body: %v", err)
			}
			if got.InstanceType != "small" || len(got.RegionIDs) != 1 || got.RegionIDs[0] != "reg-a" {
				t.Errorf("unexpected body: %+v", got)
			}
			_, _ = fmt.Fprint(w, `{"statusUrl":"https://example.test/status/add-i","samplingTime":1}`)
		default:
			http.NotFound(w, r)
		}
	})

	op, err := c.AddInstances(context.Background(), "n1", CreateInstancesPayload{
		InstanceType: "small",
		RegionIDs:    []string{"reg-a"},
	})
	if err != nil {
		t.Fatalf("AddInstances: %v", err)
	}
	if op.StatusURL == "" {
		t.Errorf("expected non-empty StatusURL, got %+v", op)
	}
}

func TestRemoveInstances(t *testing.T) {
	c, _ := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/auth/authorize":
			writeAuthResponse(w)
		case "/api/rest/v2.3/networks/standard/n1/instances":
			if r.Method != http.MethodDelete {
				t.Errorf("expected DELETE, got %s", r.Method)
			}
			body, _ := io.ReadAll(r.Body)
			var got RemoveInstancesPayload
			if err := json.Unmarshal(body, &got); err != nil {
				t.Fatalf("decode body: %v", err)
			}
			if len(got.RegionIDs) != 1 || got.RegionIDs[0] != "reg-a" {
				t.Errorf("unexpected regionIds: %+v", got.RegionIDs)
			}
			if len(got.InstanceIDs) != 2 || got.InstanceIDs[0] != "i-1" {
				t.Errorf("unexpected instanceIds: %+v", got.InstanceIDs)
			}
			_, _ = fmt.Fprint(w, `{"statusUrl":"https://example.test/status/rm-i","samplingTime":1}`)
		default:
			http.NotFound(w, r)
		}
	})

	op, err := c.RemoveInstances(context.Background(), "n1", RemoveInstancesPayload{
		RegionIDs:   []string{"reg-a"},
		InstanceIDs: []string{"i-1", "i-2"},
	})
	if err != nil {
		t.Fatalf("RemoveInstances: %v", err)
	}
	if op.StatusURL == "" {
		t.Errorf("expected non-empty StatusURL, got %+v", op)
	}
}

func TestGetInstance(t *testing.T) {
	c, _ := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/auth/authorize":
			writeAuthResponse(w)
		case "/api/rest/v2.3/networks/standard/n1/instances/i-1":
			if r.Method != http.MethodGet {
				t.Errorf("expected GET, got %s", r.Method)
			}
			_, _ = fmt.Fprint(w, `{"id":"i-1","network":"n1","region":"reg-a","instanceType":"small","ip":"1.2.3.4"}`)
		default:
			http.NotFound(w, r)
		}
	})

	got, err := c.GetInstance(context.Background(), "n1", "i-1")
	if err != nil {
		t.Fatalf("GetInstance: %v", err)
	}
	if got.ID != "i-1" || got.IP != "1.2.3.4" || got.InstanceType != "small" {
		t.Errorf("unexpected response: %+v", got)
	}
}
