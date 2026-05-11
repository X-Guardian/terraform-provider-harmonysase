// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"fmt"
	"sort"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/X-Guardian/terraform-provider-harmonysase/internal/client"
)

// networkRegionsObjectType is the element type of the computed `regions` map.
// Defined once and reused in schema + state construction.
func networkRegionsObjectType() types.ObjectType {
	gw := networkGatewayObjectType()
	return types.ObjectType{
		AttrTypes: map[string]attr.Type{
			"id":       types.StringType,
			"gateways": types.ListType{ElemType: gw},
		},
	}
}

func networkGatewayObjectType() types.ObjectType {
	return types.ObjectType{
		AttrTypes: map[string]attr.Type{
			"id":            types.StringType,
			"ip":            types.StringType,
			"dns":           types.StringType,
			"instance_type": types.StringType,
			"image_type":    types.StringType,
			"image_version": types.StringType,
			"tenant_id":     types.StringType,
		},
	}
}

// buildRegionsAttr builds the value for the `regions` computed attribute from
// the API's region list. Keyed by region name (which is what the API returns).
// If two regions share a name on the same network the second wins; that's
// not currently observed in practice.
func buildRegionsAttr(regions []client.NetworkRegion) (types.Map, error) {
	gwType := networkGatewayObjectType()
	regType := networkRegionsObjectType()

	values := make(map[string]attr.Value, len(regions))
	for i := range regions {
		region := &regions[i]
		// Build gateway list, sorted by id for stable ordering.
		sortedInstances := append([]client.NetworkInstance(nil), region.Instances...)
		sort.SliceStable(sortedInstances, func(a, b int) bool {
			return sortedInstances[a].ID < sortedInstances[b].ID
		})

		gwElems := make([]attr.Value, 0, len(sortedInstances))
		for _, ins := range sortedInstances {
			obj, diags := types.ObjectValue(gwType.AttrTypes, map[string]attr.Value{
				"id":            types.StringValue(ins.ID),
				"ip":            types.StringValue(ins.IP),
				"dns":           types.StringValue(ins.DNS),
				"instance_type": types.StringValue(ins.InstanceType),
				"image_type":    types.StringValue(ins.ImageType),
				"image_version": types.StringValue(ins.ImageVersion),
				"tenant_id":     types.StringValue(ins.TenantID),
			})
			if diags.HasError() {
				return types.MapNull(regType), fmt.Errorf("encoding gateway: %v", diags)
			}
			gwElems = append(gwElems, obj)
		}
		gwList, diags := types.ListValue(gwType, gwElems)
		if diags.HasError() {
			return types.MapNull(regType), fmt.Errorf("encoding gateway list: %v", diags)
		}

		regObj, diags := types.ObjectValue(regType.AttrTypes, map[string]attr.Value{
			"id":       types.StringValue(region.ID),
			"gateways": gwList,
		})
		if diags.HasError() {
			return types.MapNull(regType), fmt.Errorf("encoding region: %v", diags)
		}
		values[region.Name] = regObj
	}

	out, diags := types.MapValue(regType, values)
	if diags.HasError() {
		return types.MapNull(regType), fmt.Errorf("encoding regions map: %v", diags)
	}
	return out, nil
}
