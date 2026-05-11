// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/X-Guardian/terraform-provider-harmonysase/internal/client"
)

var (
	_ datasource.DataSource              = &gatewayDataSource{}
	_ datasource.DataSourceWithConfigure = &gatewayDataSource{}
)

func NewGatewayDataSource() datasource.DataSource { return &gatewayDataSource{} }

type gatewayDataSource struct {
	client *client.Client
}

type gatewayDataSourceModel struct {
	ID           types.String `tfsdk:"id"`
	NetworkID    types.String `tfsdk:"network_id"`
	RegionID     types.String `tfsdk:"region_id"`
	InstanceType types.String `tfsdk:"instance_type"`
	IP           types.String `tfsdk:"ip"`
	DNS          types.String `tfsdk:"dns"`
	ImageType    types.String `tfsdk:"image_type"`
	ImageVersion types.String `tfsdk:"image_version"`
	TenantID     types.String `tfsdk:"tenant_id"`
}

func (d *gatewayDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_standard_gateway"
}

func (d *gatewayDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Look up an existing Harmony SASE gateway (instance) by network ID and gateway ID.",
		Attributes: map[string]schema.Attribute{
			"id":            schema.StringAttribute{Required: true, MarkdownDescription: "Gateway (instance) ID."},
			"network_id":    schema.StringAttribute{Required: true, MarkdownDescription: "ID of the parent network."},
			"region_id":     schema.StringAttribute{Computed: true},
			"instance_type": schema.StringAttribute{Computed: true},
			"ip":            schema.StringAttribute{Computed: true},
			"dns":           schema.StringAttribute{Computed: true},
			"image_type":    schema.StringAttribute{Computed: true},
			"image_version": schema.StringAttribute{Computed: true},
			"tenant_id":     schema.StringAttribute{Computed: true},
		},
	}
}

func (d *gatewayDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = clientFromProviderData(req.ProviderData, &resp.Diagnostics)
}

func (d *gatewayDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data gatewayDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	ins, err := d.client.GetInstance(ctx, data.NetworkID.ValueString(), data.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Failed to read gateway", err.Error())
		return
	}

	data.RegionID = types.StringValue(ins.Region)
	data.InstanceType = types.StringValue(ins.InstanceType)
	data.IP = types.StringValue(ins.IP)
	data.DNS = types.StringValue(ins.DNS)
	data.ImageType = types.StringValue(ins.ImageType)
	data.ImageVersion = types.StringValue(ins.ImageVersion)
	data.TenantID = types.StringValue(ins.TenantID)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
