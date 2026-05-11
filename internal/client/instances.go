// SPDX-License-Identifier: MPL-2.0

package client

import (
	"context"
)

// NetworkInstance mirrors the gateway/instance object.
type NetworkInstance struct {
	ID           string          `json:"id"`
	Network      string          `json:"network"`
	Region       string          `json:"region"`
	InstanceType string          `json:"instanceType"`
	ImageType    string          `json:"imageType"`
	ImageVersion string          `json:"imageVersion"`
	DNS          string          `json:"dns"`
	IP           string          `json:"ip"`
	Tunnels      []NetworkTunnel `json:"tunnels,omitempty"`
	TenantID     string          `json:"tenantId"`
	CreatedAt    string          `json:"createdAt"`
	UpdatedAt    string          `json:"updatedAt"`
}

// NetworkTunnel is the minimal embedded tunnel reference appearing on
// NetworkInstance. The full WireguardTunnel type lives in tunnels_wireguard.go.
type NetworkTunnel struct {
	ID   string `json:"id"`
	Type string `json:"type"`
}

// CreateInstancesPayload is the body for POST /instances.
type CreateInstancesPayload struct {
	InstanceType string   `json:"instanceType"`
	RegionIDs    []string `json:"regionIds"`
}

// AddInstances is async.
func (c *Client) AddInstances(ctx context.Context, networkID string, body CreateInstancesPayload) (AsyncOperationResponse, error) {
	var out AsyncOperationResponse
	_, err := c.do(ctx, "POST", "/v2.3/networks/standard/"+networkID+"/instances", body, &out)
	return out, err
}

// RemoveInstancesPayload is the body for DELETE /instances.
type RemoveInstancesPayload struct {
	RegionIDs   []string `json:"regionIds"`
	InstanceIDs []string `json:"instanceIds,omitempty"`
}

// RemoveInstances is async.
func (c *Client) RemoveInstances(ctx context.Context, networkID string, body RemoveInstancesPayload) (AsyncOperationResponse, error) {
	var out AsyncOperationResponse
	_, err := c.do(ctx, "DELETE", "/v2.3/networks/standard/"+networkID+"/instances", body, &out)
	return out, err
}

// GetInstance is sync.
func (c *Client) GetInstance(ctx context.Context, networkID, instanceID string) (*NetworkInstance, error) {
	var out NetworkInstance
	_, err := c.do(ctx, "GET", "/v2.3/networks/standard/"+networkID+"/instances/"+instanceID, nil, &out)
	if err != nil {
		return nil, err
	}
	return &out, nil
}
