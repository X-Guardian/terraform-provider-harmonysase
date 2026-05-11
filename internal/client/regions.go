// SPDX-License-Identifier: MPL-2.0

package client

import (
	"context"
)

// HarmonySaseRegion mirrors a single entry returned by
// GET /v2.3/networks/{standard|enhanced}/harmony-sase-regions.
//
// The API also returns `className` and `objectId` fields; both are
// implementation-detail noise (`className` is always "CPRegion", `objectId`
// duplicates `id`) so we omit them.
type HarmonySaseRegion struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	DisplayName   string `json:"displayName"`
	CountryCode   string `json:"countryCode"`
	ContinentCode string `json:"continentCode"`
}

// GetStandardRegions returns the catalogue of regions available for Standard
// networks.
func (c *Client) GetStandardRegions(ctx context.Context) ([]HarmonySaseRegion, error) {
	var out []HarmonySaseRegion
	_, err := c.do(ctx, "GET", "/v2.3/networks/standard/harmony-sase-regions", nil, &out)
	return out, err
}

// GetEnhancedRegions returns the catalogue of regions available for Enhanced
// networks.
func (c *Client) GetEnhancedRegions(ctx context.Context) ([]HarmonySaseRegion, error) {
	var out []HarmonySaseRegion
	_, err := c.do(ctx, "GET", "/v2.3/networks/enhanced/harmony-sase-regions", nil, &out)
	return out, err
}
