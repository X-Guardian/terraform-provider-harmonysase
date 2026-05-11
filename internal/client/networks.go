// SPDX-License-Identifier: MPL-2.0

package client

import (
	"context"
)

// Network mirrors the API "Network" schema returned by GET network endpoints.
type Network struct {
	ID           string          `json:"id"`
	Name         string          `json:"name"`
	Subnet       string          `json:"subnet"`
	DNS          string          `json:"dns"`
	AccessType   string          `json:"accessType"`
	Applications []string        `json:"applications,omitempty"`
	Tags         []string        `json:"tags,omitempty"`
	IsDefault    bool            `json:"isDefault"`
	TenantID     string          `json:"tenantId"`
	ASN          *int64          `json:"asn,omitempty"`
	Regions      []NetworkRegion `json:"regions,omitempty"`
	CreatedAt    string          `json:"createdAt"`
	UpdatedAt    string          `json:"updatedAt"`
}

// NetworkRegion mirrors the per-network region object.
type NetworkRegion struct {
	ID        string            `json:"id"`
	Network   string            `json:"network"`
	Name      string            `json:"name"`
	DNS       string            `json:"dns"`
	Instances []NetworkInstance `json:"instances,omitempty"`
	TenantID  string            `json:"tenantId"`
	CreatedAt string            `json:"createdAt"`
	UpdatedAt string            `json:"updatedAt"`
}

// CreateRegionInNetworkPayload is the per-region body used both at network
// create and when adding regions to an existing network.
type CreateRegionInNetworkPayload struct {
	HarmonySaseRegionID string `json:"harmonySaseRegionId"`
	Idle                *bool  `json:"idle,omitempty"`
	ScaleUnits          *int64 `json:"scaleUnits,omitempty"`
	InstanceType        string `json:"instanceType,omitempty"`
}

// DeployNetworkPayload is the body for POST /networks/standard.
type DeployNetworkPayload struct {
	Network struct {
		Name   string   `json:"name"`
		Subnet string   `json:"subnet,omitempty"`
		Tags   []string `json:"tags,omitempty"`
	} `json:"network"`
	Regions []CreateRegionInNetworkPayload `json:"regions"`
}

// UpdateNetworkPayload is the body for PUT /networks/standard/{id}.
type UpdateNetworkPayload struct {
	Network struct {
		Name string   `json:"name,omitempty"`
		Tags []string `json:"tags,omitempty"`
	} `json:"network"`
}

// CreateNetwork is async — returns AsyncOperationResponse.
func (c *Client) CreateNetwork(ctx context.Context, body DeployNetworkPayload) (AsyncOperationResponse, error) {
	var out AsyncOperationResponse
	_, err := c.do(ctx, "POST", "/v2.3/networks/standard", body, &out)
	return out, err
}

// GetNetwork is sync.
func (c *Client) GetNetwork(ctx context.Context, networkID string) (*Network, error) {
	var out Network
	_, err := c.do(ctx, "GET", "/v2.3/networks/standard/"+networkID, nil, &out)
	if err != nil {
		return nil, err
	}
	return &out, nil
}

// UpdateNetwork is sync (returns the updated object).
func (c *Client) UpdateNetwork(ctx context.Context, networkID string, body UpdateNetworkPayload) (*Network, error) {
	var out Network
	_, err := c.do(ctx, "PUT", "/v2.3/networks/standard/"+networkID, body, &out)
	if err != nil {
		return nil, err
	}
	return &out, nil
}

// DeleteNetwork — the spec is silent on whether this is sync or async. We
// treat it as potentially-async by decoding the body if it's an
// AsyncOperationResponse, and returning a zero value otherwise.
func (c *Client) DeleteNetwork(ctx context.Context, networkID string) (AsyncOperationResponse, error) {
	var out AsyncOperationResponse
	_, err := c.do(ctx, "DELETE", "/v2.3/networks/standard/"+networkID, nil, &out)
	return out, err
}

// AddRegionsToNetwork is async.
func (c *Client) AddRegionsToNetwork(ctx context.Context, networkID string, regions []CreateRegionInNetworkPayload) (AsyncOperationResponse, error) {
	var out AsyncOperationResponse
	_, err := c.do(ctx, "PUT", "/v2.3/networks/standard/"+networkID+"/regions", regions, &out)
	return out, err
}

// RemoveRegionsPayload is the body for DELETE .../regions.
type RemoveRegionsPayload struct {
	RegionIDs []string `json:"regionIds"`
}

// RemoveRegionsFromNetwork is async.
func (c *Client) RemoveRegionsFromNetwork(ctx context.Context, networkID string, regionIDs []string) (AsyncOperationResponse, error) {
	var out AsyncOperationResponse
	_, err := c.do(ctx, "DELETE", "/v2.3/networks/standard/"+networkID+"/regions", RemoveRegionsPayload{RegionIDs: regionIDs}, &out)
	return out, err
}

// GetRegion is sync.
func (c *Client) GetRegion(ctx context.Context, networkID, regionID string) (*NetworkRegion, error) {
	var out NetworkRegion
	_, err := c.do(ctx, "GET", "/v2.3/networks/standard/"+networkID+"/regions/"+regionID, nil, &out)
	if err != nil {
		return nil, err
	}
	return &out, nil
}
