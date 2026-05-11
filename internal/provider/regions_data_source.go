// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/X-Guardian/terraform-provider-harmonysase/internal/client"
)

var (
	_ datasource.DataSource              = &regionsDataSource{}
	_ datasource.DataSourceWithConfigure = &regionsDataSource{}
)

// NewRegionsDataSource returns a new regions data source.
func NewRegionsDataSource() datasource.DataSource { return &regionsDataSource{} }

type regionsDataSource struct {
	client *client.Client
}

type regionsDataSourceModel struct {
	NetworkType   types.String `tfsdk:"network_type"`
	Name          types.String `tfsdk:"name"`
	DisplayName   types.String `tfsdk:"display_name"`
	CountryCode   types.String `tfsdk:"country_code"`
	ContinentCode types.String `tfsdk:"continent_code"`
	Regions       types.List   `tfsdk:"regions"`
	ByName        types.Map    `tfsdk:"by_name"`
	ByDisplayName types.Map    `tfsdk:"by_display_name"`
}

// regionObjectType is the element type for the `regions` list and for both
// `by_*` maps. Defined once so schema and value construction stay in sync.
func regionObjectType() types.ObjectType {
	return types.ObjectType{
		AttrTypes: map[string]attr.Type{
			"id":             types.StringType,
			"name":           types.StringType,
			"display_name":   types.StringType,
			"country_code":   types.StringType,
			"continent_code": types.StringType,
		},
	}
}

func (d *regionsDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_regions"
}

func (d *regionsDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	regionAttrs := map[string]schema.Attribute{
		"id":             schema.StringAttribute{Computed: true, MarkdownDescription: "Catalogue region ID. This is the value to put into `harmony_sase_region_id`."},
		"name":           schema.StringAttribute{Computed: true, MarkdownDescription: "API `name` field. Often but not always equal to `display_name`."},
		"display_name":   schema.StringAttribute{Computed: true, MarkdownDescription: "User-facing label as shown in the SASE admin UI."},
		"country_code":   schema.StringAttribute{Computed: true},
		"continent_code": schema.StringAttribute{Computed: true},
	}

	resp.Schema = schema.Schema{
		MarkdownDescription: "Catalogue of Harmony SASE regions available for Standard or Enhanced networks. Use this to look up `harmony_sase_region_id` values without hardcoding catalogue IDs in HCL.\n\nNote: the API's `name` and `display_name` fields can diverge for some regions (e.g. `displayName=\"Tokyo 1\"` vs `name=\"Tokyo\"`). Both are exposed here; pick whichever matches what you'll see at the source.",
		Attributes: map[string]schema.Attribute{
			"network_type": schema.StringAttribute{
				MarkdownDescription: "Which catalogue to fetch. One of `standard` (default) or `enhanced`.",
				Optional:            true,
				Validators: []validator.String{
					stringvalidator.OneOf("standard", "enhanced"),
				},
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "Filter to a single region whose API `name` matches (case-insensitive, exact).",
				Optional:            true,
			},
			"display_name": schema.StringAttribute{
				MarkdownDescription: "Filter to a single region whose `display_name` matches (case-insensitive, exact).",
				Optional:            true,
			},
			"country_code": schema.StringAttribute{
				MarkdownDescription: "Filter to all regions with the given ISO country code (case-insensitive, e.g. `GB`).",
				Optional:            true,
			},
			"continent_code": schema.StringAttribute{
				MarkdownDescription: "Filter to all regions with the given continent code (e.g. `EU`, `NA`, `AS`).",
				Optional:            true,
			},
			"regions": schema.ListNestedAttribute{
				MarkdownDescription: "All regions matching the filters, in the order returned by the API.",
				Computed:            true,
				NestedObject:        schema.NestedAttributeObject{Attributes: regionAttrs},
			},
			"by_name": schema.MapNestedAttribute{
				MarkdownDescription: "Convenience map of matching regions keyed by API `name`. May contain fewer entries than `regions` if duplicate names exist (none observed in practice).",
				Computed:            true,
				NestedObject:        schema.NestedAttributeObject{Attributes: regionAttrs},
			},
			"by_display_name": schema.MapNestedAttribute{
				MarkdownDescription: "Convenience map of matching regions keyed by `display_name`.",
				Computed:            true,
				NestedObject:        schema.NestedAttributeObject{Attributes: regionAttrs},
			},
		},
	}
}

func (d *regionsDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = clientFromProviderData(req.ProviderData, &resp.Diagnostics)
}

func (d *regionsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data regionsDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	networkType := strings.ToLower(strings.TrimSpace(data.NetworkType.ValueString()))
	if networkType == "" {
		networkType = "standard"
	}

	var (
		all []client.HarmonySaseRegion
		err error
	)
	switch networkType {
	case "standard":
		all, err = d.client.GetStandardRegions(ctx)
	case "enhanced":
		all, err = d.client.GetEnhancedRegions(ctx)
	default:
		// Already validated by the schema; defensive.
		resp.Diagnostics.AddError("Invalid network_type", fmt.Sprintf("expected standard or enhanced, got %q", networkType))
		return
	}
	if err != nil {
		resp.Diagnostics.AddError("Failed to fetch region catalogue", err.Error())
		return
	}

	wantName := strings.ToLower(strings.TrimSpace(data.Name.ValueString()))
	wantDisplay := strings.ToLower(strings.TrimSpace(data.DisplayName.ValueString()))
	wantCountry := strings.ToLower(strings.TrimSpace(data.CountryCode.ValueString()))
	wantContinent := strings.ToLower(strings.TrimSpace(data.ContinentCode.ValueString()))

	filtered := make([]client.HarmonySaseRegion, 0, len(all))
	for _, r := range all {
		if wantName != "" && strings.ToLower(r.Name) != wantName {
			continue
		}
		if wantDisplay != "" && strings.ToLower(r.DisplayName) != wantDisplay {
			continue
		}
		if wantCountry != "" && strings.ToLower(r.CountryCode) != wantCountry {
			continue
		}
		if wantContinent != "" && strings.ToLower(r.ContinentCode) != wantContinent {
			continue
		}
		filtered = append(filtered, r)
	}

	regionType := regionObjectType()
	objAttrs := regionType.AttrTypes

	listElems := make([]attr.Value, 0, len(filtered))
	byName := make(map[string]attr.Value, len(filtered))
	byDisplay := make(map[string]attr.Value, len(filtered))
	for _, r := range filtered {
		obj, diags := types.ObjectValue(objAttrs, map[string]attr.Value{
			"id":             types.StringValue(r.ID),
			"name":           types.StringValue(r.Name),
			"display_name":   types.StringValue(r.DisplayName),
			"country_code":   types.StringValue(r.CountryCode),
			"continent_code": types.StringValue(r.ContinentCode),
		})
		if diags.HasError() {
			resp.Diagnostics.Append(diags...)
			return
		}
		listElems = append(listElems, obj)
		byName[r.Name] = obj
		byDisplay[r.DisplayName] = obj
	}

	regionsList, diags := types.ListValue(regionType, listElems)
	resp.Diagnostics.Append(diags...)
	byNameMap, diags := types.MapValue(regionType, byName)
	resp.Diagnostics.Append(diags...)
	byDisplayMap, diags := types.MapValue(regionType, byDisplay)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	data.Regions = regionsList
	data.ByName = byNameMap
	data.ByDisplayName = byDisplayMap

	// Echo back normalised network_type so state matches what we used.
	data.NetworkType = types.StringValue(networkType)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
