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

// CreateInstancesPayload is the body for POST /instances. The endpoint adds a single gateway to one region per call;
// callers scaling by more than one must loop. Gateways inherit their instance type from the region.
type CreateInstancesPayload struct {
	RegionID string `json:"regionId"`
	Idle     bool   `json:"idle"`
}

// AddInstances is async. It adds one gateway to the payload's region.
func (c *Client) AddInstances(ctx context.Context, networkID string, body CreateInstancesPayload) (AsyncOperationResponse, error) {
	var out AsyncOperationResponse
	_, err := c.do(ctx, "POST", "/v2.3/networks/standard/"+networkID+"/instances", body, &out)
	return out, err
}

// RemoveInstancesPayload is the body for DELETE /instances. Gateways are addressed as a nested list grouped by region.
type RemoveInstancesPayload struct {
	Regions []RemoveInstancesRegion `json:"regions"`
}

// RemoveInstancesRegion names one region and the gateways to remove from it.
type RemoveInstancesRegion struct {
	RegionID  string                    `json:"regionId"`
	Instances []RemoveInstancesInstance `json:"instances,omitempty"`
}

// RemoveInstancesInstance identifies a single gateway to remove.
type RemoveInstancesInstance struct {
	ID string `json:"id"`
}

// NewRemoveInstancesPayload builds the nested removal body for a set of gateway IDs within a single region.
func NewRemoveInstancesPayload(regionID string, instanceIDs []string) RemoveInstancesPayload {
	instances := make([]RemoveInstancesInstance, 0, len(instanceIDs))
	for _, id := range instanceIDs {
		instances = append(instances, RemoveInstancesInstance{ID: id})
	}
	return RemoveInstancesPayload{
		Regions: []RemoveInstancesRegion{{RegionID: regionID, Instances: instances}},
	}
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
