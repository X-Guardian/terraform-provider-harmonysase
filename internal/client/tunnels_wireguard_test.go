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

func TestCreateWireguardTunnel(t *testing.T) {
	c, _ := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/auth/authorize":
			writeAuthResponse(w)
		case "/api/rest/v2.3/networks/standard/n1/tunnels/wireguard":
			if r.Method != http.MethodPost {
				t.Errorf("expected POST, got %s", r.Method)
			}
			body, _ := io.ReadAll(r.Body)
			var got CreateWireguardTunnelPayload
			if err := json.Unmarshal(body, &got); err != nil {
				t.Fatalf("decode body: %v", err)
			}
			if got.TunnelName != "wg-1" || got.GatewayID != "gw-a" {
				t.Errorf("unexpected body: %+v", got)
			}
			_, _ = fmt.Fprint(w, `{"statusUrl":"https://example.test/status/wg-create","samplingTime":1}`)
		default:
			http.NotFound(w, r)
		}
	})

	op, err := c.CreateWireguardTunnel(context.Background(), "n1", CreateWireguardTunnelPayload{
		TunnelName:     "wg-1",
		RegionID:       "reg-a",
		GatewayID:      "gw-a",
		RemoteEndpoint: "10.0.0.1",
		RemoteSubnets:  []string{"192.168.1.0/24"},
	})
	if err != nil {
		t.Fatalf("CreateWireguardTunnel: %v", err)
	}
	if op.StatusURL == "" {
		t.Errorf("expected non-empty StatusURL, got %+v", op)
	}
}

func TestGetWireguardTunnel(t *testing.T) {
	c, _ := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/auth/authorize":
			writeAuthResponse(w)
		case "/api/rest/v2.3/networks/standard/n1/tunnels/wireguard/t-1":
			if r.Method != http.MethodGet {
				t.Errorf("expected GET, got %s", r.Method)
			}
			_, _ = fmt.Fprint(w, `{"tunnelID":"t-1","tunnelName":"wg-1","regionID":"reg-a","gatewayID":"gw-a","remoteEndpoint":"10.0.0.1","remoteSubnets":["192.168.1.0/24"]}`)
		default:
			http.NotFound(w, r)
		}
	})

	got, err := c.GetWireguardTunnel(context.Background(), "n1", "t-1")
	if err != nil {
		t.Fatalf("GetWireguardTunnel: %v", err)
	}
	if got.TunnelID != "t-1" || got.RemoteEndpoint != "10.0.0.1" || len(got.RemoteSubnets) != 1 {
		t.Errorf("unexpected response: %+v", got)
	}
}

func TestUpdateWireguardTunnel(t *testing.T) {
	c, _ := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/auth/authorize":
			writeAuthResponse(w)
		case "/api/rest/v2.3/networks/standard/n1/tunnels/wireguard/t-1":
			if r.Method != http.MethodPut {
				t.Errorf("expected PUT, got %s", r.Method)
			}
			body, _ := io.ReadAll(r.Body)
			var got UpdateWireguardTunnelPayload
			if err := json.Unmarshal(body, &got); err != nil {
				t.Fatalf("decode body: %v", err)
			}
			if got.RemoteEndpoint != "10.0.0.2" {
				t.Errorf("unexpected body: %+v", got)
			}
			_, _ = fmt.Fprint(w, `{"statusUrl":"https://example.test/status/wg-update","samplingTime":1}`)
		default:
			http.NotFound(w, r)
		}
	})

	op, err := c.UpdateWireguardTunnel(context.Background(), "n1", "t-1", UpdateWireguardTunnelPayload{
		RemoteEndpoint: "10.0.0.2",
		RemoteSubnets:  []string{"192.168.2.0/24"},
	})
	if err != nil {
		t.Fatalf("UpdateWireguardTunnel: %v", err)
	}
	if op.StatusURL == "" {
		t.Errorf("expected non-empty StatusURL, got %+v", op)
	}
}

func TestDeleteWireguardTunnel(t *testing.T) {
	c, _ := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/auth/authorize":
			writeAuthResponse(w)
		case "/api/rest/v2.3/networks/standard/n1/tunnels/wireguard/t-1":
			if r.Method != http.MethodDelete {
				t.Errorf("expected DELETE, got %s", r.Method)
			}
			_, _ = fmt.Fprint(w, `{"statusUrl":"https://example.test/status/wg-delete","samplingTime":1}`)
		default:
			http.NotFound(w, r)
		}
	})

	op, err := c.DeleteWireguardTunnel(context.Background(), "n1", "t-1")
	if err != nil {
		t.Fatalf("DeleteWireguardTunnel: %v", err)
	}
	if op.StatusURL == "" {
		t.Errorf("expected non-empty StatusURL, got %+v", op)
	}
}
