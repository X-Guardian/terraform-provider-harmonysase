// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"time"

	"github.com/hashicorp/terraform-plugin-framework-timeouts/resource/timeouts"
	"github.com/hashicorp/terraform-plugin-framework-validators/setvalidator"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	"github.com/X-Guardian/terraform-provider-harmonysase/internal/client"
)

var (
	_ resource.Resource                = &wireguardTunnelResource{}
	_ resource.ResourceWithConfigure   = &wireguardTunnelResource{}
	_ resource.ResourceWithImportState = &wireguardTunnelResource{}
)

const (
	defaultTunnelCreateTimeout = 15 * time.Minute
	defaultTunnelUpdateTimeout = 10 * time.Minute
	defaultTunnelDeleteTimeout = 10 * time.Minute
)

func NewWireguardTunnelResource() resource.Resource { return &wireguardTunnelResource{} }

type wireguardTunnelResource struct {
	client *client.Client
}

type wireguardTunnelResourceModel struct {
	ID             types.String   `tfsdk:"id"`
	NetworkID      types.String   `tfsdk:"network_id"`
	RegionID       types.String   `tfsdk:"region_id"`
	GatewayID      types.String   `tfsdk:"gateway_id"`
	Name           types.String   `tfsdk:"name"`
	RemoteEndpoint types.String   `tfsdk:"remote_endpoint"`
	RemoteSubnets  types.Set      `tfsdk:"remote_subnets"`
	Timeouts       timeouts.Value `tfsdk:"timeouts"`
}

func (r *wireguardTunnelResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_standard_wireguard_tunnel"
}

func (r *wireguardTunnelResource) Schema(ctx context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	requiresReplaceStr := []planmodifier.String{stringplanmodifier.RequiresReplace()}
	resp.Schema = schema.Schema{
		MarkdownDescription: "WireGuard site-to-site tunnel attached to a specific gateway in a Harmony SASE standard network.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "Tunnel ID assigned by the API.",
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"network_id": schema.StringAttribute{
				MarkdownDescription: "ID of the parent network.",
				Required:            true,
				PlanModifiers:       requiresReplaceStr,
			},
			"region_id": schema.StringAttribute{
				MarkdownDescription: "Runtime region ID the tunnel is attached to. Changing forces replacement.",
				Required:            true,
				PlanModifiers:       requiresReplaceStr,
			},
			"gateway_id": schema.StringAttribute{
				MarkdownDescription: "Gateway (instance) ID the tunnel is attached to. Changing forces replacement.",
				Required:            true,
				PlanModifiers:       requiresReplaceStr,
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "Tunnel name (interface name on the gateway). Changing forces replacement.",
				Required:            true,
				PlanModifiers:       requiresReplaceStr,
			},
			"remote_endpoint": schema.StringAttribute{
				MarkdownDescription: "Public IPv4 of the remote tunnel endpoint. Updatable in place.",
				Required:            true,
			},
			"remote_subnets": schema.SetAttribute{
				MarkdownDescription: "CIDR(s) reachable through the tunnel. Updatable in place.",
				ElementType:         types.StringType,
				Required:            true,
				Validators: []validator.Set{
					setvalidator.SizeAtLeast(1),
					NonOverlappingCIDRs(),
				},
			},
		},
		Blocks: map[string]schema.Block{
			"timeouts": timeouts.Block(ctx, timeouts.Opts{Create: true, Update: true, Delete: true}),
		},
	}
}

func (r *wireguardTunnelResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = clientFromProviderData(req.ProviderData, &resp.Diagnostics)
}

func (r *wireguardTunnelResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan wireguardTunnelResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	createTimeout, diags := plan.Timeouts.Create(ctx, defaultTunnelCreateTimeout)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	ctx, cancel := context.WithTimeout(ctx, createTimeout)
	defer cancel()

	var subnets []string
	resp.Diagnostics.Append(plan.RemoteSubnets.ElementsAs(ctx, &subnets, false)...)
	if resp.Diagnostics.HasError() {
		return
	}

	body := client.CreateWireguardTunnelPayload{
		TunnelName:     plan.Name.ValueString(),
		RegionID:       plan.RegionID.ValueString(),
		GatewayID:      plan.GatewayID.ValueString(),
		RemoteEndpoint: plan.RemoteEndpoint.ValueString(),
		RemoteSubnets:  subnets,
	}
	op, err := r.client.CreateWireguardTunnel(ctx, plan.NetworkID.ValueString(), body)
	if err != nil {
		resp.Diagnostics.AddError("Failed to create tunnel", err.Error())
		return
	}
	resultRaw, err := r.client.WaitForOperation(ctx, op)
	if err != nil {
		resp.Diagnostics.AddError("Tunnel create did not complete", err.Error())
		return
	}
	// The async result envelope carries the tunnel object. Field name observed
	// as `tunnelID` on the standalone GET; the create result may use that or a
	// generic `id` — accept either.
	var result struct {
		TunnelID string `json:"tunnelID"`
		ID       string `json:"id"`
	}
	if len(resultRaw) > 0 {
		_ = jsonUnmarshal(resultRaw, &result)
	}
	id := result.TunnelID
	if id == "" {
		id = result.ID
	}
	if id == "" {
		resp.Diagnostics.AddError(
			"Could not determine tunnel ID after create",
			"The async status payload did not contain a `tunnelID` or `id` field.",
		)
		return
	}

	if err := r.refreshState(ctx, &plan, plan.NetworkID.ValueString(), id); err != nil {
		resp.Diagnostics.AddError("Failed to read tunnel after create", err.Error())
		return
	}
	tflog.Debug(ctx, "created harmonysase standard wireguard tunnel", map[string]any{"id": id, "network_id": plan.NetworkID.ValueString()})
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *wireguardTunnelResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state wireguardTunnelResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.refreshState(ctx, &state, state.NetworkID.ValueString(), state.ID.ValueString()); err != nil {
		if client.IsNotFound(err) {
			tflog.Warn(ctx, "harmonysase standard wireguard tunnel not found, removing from state", map[string]any{"id": state.ID.ValueString(), "network_id": state.NetworkID.ValueString()})
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Failed to read tunnel", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *wireguardTunnelResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state wireguardTunnelResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	updateTimeout, diags := plan.Timeouts.Update(ctx, defaultTunnelUpdateTimeout)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	ctx, cancel := context.WithTimeout(ctx, updateTimeout)
	defer cancel()

	var subnets []string
	resp.Diagnostics.Append(plan.RemoteSubnets.ElementsAs(ctx, &subnets, false)...)
	if resp.Diagnostics.HasError() {
		return
	}
	op, err := r.client.UpdateWireguardTunnel(ctx, state.NetworkID.ValueString(), state.ID.ValueString(), client.UpdateWireguardTunnelPayload{
		RemoteEndpoint: plan.RemoteEndpoint.ValueString(),
		RemoteSubnets:  subnets,
	})
	if err != nil {
		resp.Diagnostics.AddError("Failed to update tunnel", err.Error())
		return
	}
	if op.StatusURL != "" {
		if _, err := r.client.WaitForOperation(ctx, op); err != nil {
			resp.Diagnostics.AddError("Tunnel update did not complete", err.Error())
			return
		}
	}
	if err := r.refreshState(ctx, &plan, state.NetworkID.ValueString(), state.ID.ValueString()); err != nil {
		resp.Diagnostics.AddError("Failed to read tunnel after update", err.Error())
		return
	}
	tflog.Debug(ctx, "updated harmonysase standard wireguard tunnel", map[string]any{"id": state.ID.ValueString(), "network_id": state.NetworkID.ValueString()})
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *wireguardTunnelResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state wireguardTunnelResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	deleteTimeout, diags := state.Timeouts.Delete(ctx, defaultTunnelDeleteTimeout)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	ctx, cancel := context.WithTimeout(ctx, deleteTimeout)
	defer cancel()

	op, err := r.client.DeleteWireguardTunnel(ctx, state.NetworkID.ValueString(), state.ID.ValueString())
	if err != nil {
		if client.IsNotFound(err) {
			return
		}
		resp.Diagnostics.AddError("Failed to delete tunnel", err.Error())
		return
	}
	if op.StatusURL != "" {
		if _, err := r.client.WaitForOperation(ctx, op); err != nil {
			resp.Diagnostics.AddError("Tunnel delete did not complete", err.Error())
			return
		}
	}
	tflog.Debug(ctx, "deleted harmonysase standard wireguard tunnel", map[string]any{"id": state.ID.ValueString(), "network_id": state.NetworkID.ValueString()})
}

func (r *wireguardTunnelResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	parts := splitImportID(req.ID, 2)
	if parts == nil {
		resp.Diagnostics.AddError("Invalid import ID", "Expected format: network_id,tunnel_id")
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("network_id"), parts[0])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), parts[1])...)
}

func (r *wireguardTunnelResource) refreshState(ctx context.Context, m *wireguardTunnelResourceModel, networkID, tunnelID string) error {
	t, err := r.client.GetWireguardTunnel(ctx, networkID, tunnelID)
	if err != nil {
		return err
	}
	m.ID = types.StringValue(t.TunnelID)
	m.NetworkID = types.StringValue(networkID)
	m.RegionID = types.StringValue(t.RegionID)
	m.GatewayID = types.StringValue(t.GatewayID)
	m.Name = types.StringValue(t.TunnelName)
	m.RemoteEndpoint = types.StringValue(t.RemoteEndpoint)

	subnetSet, diags := types.SetValueFrom(ctx, types.StringType, t.RemoteSubnets)
	if diags.HasError() {
		return errFromDiags(diags)
	}
	m.RemoteSubnets = subnetSet
	return nil
}
