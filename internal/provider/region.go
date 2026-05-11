// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"sort"

	"github.com/X-Guardian/terraform-provider-harmonysase/internal/client"
)

// oldestRegion returns the region whose oldest instance has the earliest
// createdAt timestamp; falls back to the region's own createdAt when its
// instances list is empty. Returns nil when given an empty slice.
//
// Used by the standard_network and standard_gateway resources to identify
// the "initial" region attached to a network.
func oldestRegion(regions []client.NetworkRegion) *client.NetworkRegion {
	if len(regions) == 0 {
		return nil
	}
	type pair struct {
		region *client.NetworkRegion
		oldest string
	}
	var pairs []pair
	for i := range regions {
		oldest := regions[i].CreatedAt
		for _, ins := range regions[i].Instances {
			if ins.CreatedAt != "" && (oldest == "" || ins.CreatedAt < oldest) {
				oldest = ins.CreatedAt
			}
		}
		pairs = append(pairs, pair{region: &regions[i], oldest: oldest})
	}
	sort.SliceStable(pairs, func(i, j int) bool { return pairs[i].oldest < pairs[j].oldest })
	return pairs[0].region
}
