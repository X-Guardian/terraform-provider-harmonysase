// SPDX-License-Identifier: MPL-2.0

package client

import (
	"context"
)

// WireguardTunnel mirrors the GET /tunnels/wireguard/{id} response.
//
// Note: the API exposes WireGuard tunnels via two endpoints with different
// field-name conventions. This struct matches the standalone CRUD endpoints
// (POST/GET/PUT/DELETE /tunnels/wireguard/{id}). The same tunnel embedded in
// a `GET /networks/{id}` response uses different names (leftEndpoint,
// leftAllowedIP, interfaceName) — we don't decode into this struct from
// there.
type WireguardTunnel struct {
	TunnelID       string   `json:"tunnelID"`
	TunnelName     string   `json:"tunnelName"`
	RegionID       string   `json:"regionID"`
	GatewayID      string   `json:"gatewayID"`
	RemoteEndpoint string   `json:"remoteEndpoint"`
	RemoteSubnets  []string `json:"remoteSubnets"`
	CreatedAt      string   `json:"createdAt,omitempty"`
	UpdatedAt      string   `json:"updatedAt,omitempty"`
}

// CreateWireguardTunnelPayload is the POST body. All fields required.
type CreateWireguardTunnelPayload struct {
	TunnelName     string   `json:"tunnelName"`
	RegionID       string   `json:"regionID"`
	GatewayID      string   `json:"gatewayID"`
	RemoteEndpoint string   `json:"remoteEndpoint"`
	RemoteSubnets  []string `json:"remoteSubnets"`
}

// UpdateWireguardTunnelPayload is the PUT body. The API only allows changing
// remoteEndpoint and remoteSubnets; tunnelName/regionID/gatewayID are
// immutable post-create.
type UpdateWireguardTunnelPayload struct {
	RemoteEndpoint string   `json:"remoteEndpoint"`
	RemoteSubnets  []string `json:"remoteSubnets"`
}

func (c *Client) CreateWireguardTunnel(ctx context.Context, networkID string, body CreateWireguardTunnelPayload) (AsyncOperationResponse, error) {
	var out AsyncOperationResponse
	_, err := c.do(ctx, "POST", "/v2.3/networks/standard/"+networkID+"/tunnels/wireguard", body, &out)
	return out, err
}

func (c *Client) GetWireguardTunnel(ctx context.Context, networkID, tunnelID string) (*WireguardTunnel, error) {
	var out WireguardTunnel
	_, err := c.do(ctx, "GET", "/v2.3/networks/standard/"+networkID+"/tunnels/wireguard/"+tunnelID, nil, &out)
	if err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) UpdateWireguardTunnel(ctx context.Context, networkID, tunnelID string, body UpdateWireguardTunnelPayload) (AsyncOperationResponse, error) {
	var out AsyncOperationResponse
	_, err := c.do(ctx, "PUT", "/v2.3/networks/standard/"+networkID+"/tunnels/wireguard/"+tunnelID, body, &out)
	return out, err
}

func (c *Client) DeleteWireguardTunnel(ctx context.Context, networkID, tunnelID string) (AsyncOperationResponse, error) {
	var out AsyncOperationResponse
	_, err := c.do(ctx, "DELETE", "/v2.3/networks/standard/"+networkID+"/tunnels/wireguard/"+tunnelID, nil, &out)
	return out, err
}
