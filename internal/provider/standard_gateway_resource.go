// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"time"

	"github.com/hashicorp/terraform-plugin-framework-timeouts/resource/timeouts"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	"github.com/X-Guardian/terraform-provider-harmonysase/internal/client"
)

var (
	_ resource.Resource                = &gatewayResource{}
	_ resource.ResourceWithConfigure   = &gatewayResource{}
	_ resource.ResourceWithImportState = &gatewayResource{}
)

const (
	defaultGatewayCreateTimeout = 20 * time.Minute
	defaultGatewayDeleteTimeout = 15 * time.Minute
)

func NewGatewayResource() resource.Resource { return &gatewayResource{} }

type gatewayResource struct {
	client *client.Client
}

type gatewayResourceModel struct {
	ID                  types.String   `tfsdk:"id"`
	NetworkID           types.String   `tfsdk:"network_id"`
	HarmonySaseRegionID types.String   `tfsdk:"harmony_sase_region_id"`
	InstanceType        types.String   `tfsdk:"instance_type"`
	Idle                types.Bool     `tfsdk:"idle"`
	RegionID            types.String   `tfsdk:"region_id"`
	IP                  types.String   `tfsdk:"ip"`
	DNS                 types.String   `tfsdk:"dns"`
	ImageType           types.String   `tfsdk:"image_type"`
	ImageVersion        types.String   `tfsdk:"image_version"`
	TenantID            types.String   `tfsdk:"tenant_id"`
	Timeouts            timeouts.Value `tfsdk:"timeouts"`
}

func (r *gatewayResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_standard_gateway"
}

func (r *gatewayResource) Schema(ctx context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	requiresReplaceStr := []planmodifier.String{stringplanmodifier.RequiresReplace()}
	resp.Schema = schema.Schema{
		MarkdownDescription: "An additional gateway in a Harmony SASE network. The first gateway in the network's initial region is managed by `harmonysase_network`; everything else uses this resource. If this is the first gateway in its SASE region, the region itself is attached to the network as part of create and removed as part of delete (when no other gateways remain in it).",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "Gateway (instance) ID.",
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"network_id": schema.StringAttribute{
				MarkdownDescription: "ID of the parent network.",
				Required:            true,
				PlanModifiers:       requiresReplaceStr,
			},
			"harmony_sase_region_id": schema.StringAttribute{
				MarkdownDescription: "ID of the SASE region to place this gateway in.",
				Required:            true,
				PlanModifiers:       requiresReplaceStr,
			},
			"instance_type": schema.StringAttribute{
				MarkdownDescription: "Instance type. Required when this gateway causes a new region to be attached; otherwise inherited from the existing region.",
				Optional:            true,
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace(), stringplanmodifier.UseStateForUnknown()},
			},
			"idle": schema.BoolAttribute{
				MarkdownDescription: "Create the gateway in idle/disabled state. Only meaningful when this gateway causes a new region to be attached. Defaults to false.",
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(false),
				PlanModifiers:       []planmodifier.Bool{},
			},
			"region_id": schema.StringAttribute{
				MarkdownDescription: "Runtime region ID assigned by the API.",
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"ip": schema.StringAttribute{
				MarkdownDescription: "Public IP of the gateway.",
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"dns": schema.StringAttribute{
				Computed:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"image_type": schema.StringAttribute{
				Computed:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"image_version": schema.StringAttribute{
				Computed: true,
			},
			"tenant_id": schema.StringAttribute{
				Computed:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
		},
		Blocks: map[string]schema.Block{
			"timeouts": timeouts.Block(ctx, timeouts.Opts{Create: true, Delete: true}),
		},
	}
}

func (r *gatewayResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = clientFromProviderData(req.ProviderData, &resp.Diagnostics)
}

func (r *gatewayResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan gatewayResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	createTimeout, diags := plan.Timeouts.Create(ctx, defaultGatewayCreateTimeout)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	ctx, cancel := context.WithTimeout(ctx, createTimeout)
	defer cancel()

	networkID := plan.NetworkID.ValueString()
	want := plan.HarmonySaseRegionID.ValueString()

	// Snapshot the network to find out whether the requested region is
	// already attached.
	net, err := r.client.GetNetwork(ctx, networkID)
	if err != nil {
		resp.Diagnostics.AddError("Failed to read parent network", err.Error())
		return
	}
	var existingRegion *client.NetworkRegion
	for i := range net.Regions {
		if net.Regions[i].Name == want || net.Regions[i].ID == want {
			existingRegion = &net.Regions[i]
			break
		}
	}

	var newGatewayID string
	if existingRegion == nil {
		// Region not attached → PUT /regions creates the region AND its first gateway in one step. instance_type is
		// computed back from the created gateway.
		one := int64(1)
		op, err := r.client.AddRegionToNetwork(ctx, networkID, client.CreateRegionInNetworkPayload{
			HarmonySaseRegionID: want,
			ScaleUnits:          &one,
			Idle:                plan.Idle.ValueBool(),
		})
		if err != nil {
			resp.Diagnostics.AddError("Failed to attach region", err.Error())
			return
		}
		if _, err := r.client.WaitForOperation(ctx, op); err != nil {
			resp.Diagnostics.AddError("Region attach did not complete", err.Error())
			return
		}
		// Locate the new gateway by re-reading the network.
		net2, err := r.client.GetNetwork(ctx, networkID)
		if err != nil {
			resp.Diagnostics.AddError("Failed to re-read network", err.Error())
			return
		}
		for i := range net2.Regions {
			if net2.Regions[i].Name == want || net2.Regions[i].ID == want {
				if len(net2.Regions[i].Instances) > 0 {
					newGatewayID = net2.Regions[i].Instances[0].ID
				}
				break
			}
		}
	} else {
		// Region already attached → POST /instances. Identify the new
		// gateway by diffing instance IDs before vs after.
		before := map[string]struct{}{}
		for _, ins := range existingRegion.Instances {
			before[ins.ID] = struct{}{}
		}

		op, err := r.client.AddInstances(ctx, networkID, client.CreateInstancesPayload{
			RegionID: existingRegion.ID,
			Idle:     plan.Idle.ValueBool(),
		})
		if err != nil {
			resp.Diagnostics.AddError("Failed to add gateway", err.Error())
			return
		}
		if _, err := r.client.WaitForOperation(ctx, op); err != nil {
			resp.Diagnostics.AddError("Gateway add did not complete", err.Error())
			return
		}
		net2, err := r.client.GetNetwork(ctx, networkID)
		if err != nil {
			resp.Diagnostics.AddError("Failed to re-read network", err.Error())
			return
		}
		for i := range net2.Regions {
			if net2.Regions[i].ID != existingRegion.ID {
				continue
			}
			for _, ins := range net2.Regions[i].Instances {
				if _, seen := before[ins.ID]; !seen {
					newGatewayID = ins.ID
					break
				}
			}
			break
		}
	}

	if newGatewayID == "" {
		resp.Diagnostics.AddError(
			"Could not identify new gateway",
			"The gateway create succeeded but no new instance was visible after re-reading the network.",
		)
		return
	}

	if err := r.refreshState(ctx, &plan, networkID, newGatewayID); err != nil {
		resp.Diagnostics.AddError("Failed to read gateway after create", err.Error())
		return
	}
	tflog.Debug(ctx, "created harmonysase standard gateway", map[string]any{"id": newGatewayID, "network_id": networkID})
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *gatewayResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state gatewayResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.refreshState(ctx, &state, state.NetworkID.ValueString(), state.ID.ValueString()); err != nil {
		if client.IsNotFound(err) {
			tflog.Warn(ctx, "harmonysase standard gateway not found, removing from state", map[string]any{"id": state.ID.ValueString(), "network_id": state.NetworkID.ValueString()})
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Failed to read gateway", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *gatewayResource) Update(_ context.Context, _ resource.UpdateRequest, _ *resource.UpdateResponse) {
	// All mutable fields are RequiresReplace; framework will not call Update.
}

func (r *gatewayResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state gatewayResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	deleteTimeout, diags := state.Timeouts.Delete(ctx, defaultGatewayDeleteTimeout)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	ctx, cancel := context.WithTimeout(ctx, deleteTimeout)
	defer cancel()

	networkID := state.NetworkID.ValueString()
	regionID := state.RegionID.ValueString()
	gatewayID := state.ID.ValueString()

	if regionID == "" {
		// Recover from old/imported state without region_id.
		ins, err := r.client.GetInstance(ctx, networkID, gatewayID)
		if err != nil {
			if client.IsNotFound(err) {
				return
			}
			resp.Diagnostics.AddError("Failed to read gateway for delete", err.Error())
			return
		}
		regionID = ins.Region
	}

	op, err := r.client.RemoveInstances(ctx, networkID,
		client.NewRemoveInstancesPayload(regionID, []string{gatewayID}))
	if err != nil {
		if client.IsNotFound(err) {
			return
		}
		resp.Diagnostics.AddError("Failed to delete gateway", err.Error())
		return
	}
	if op.StatusURL != "" {
		if _, err := r.client.WaitForOperation(ctx, op); err != nil {
			resp.Diagnostics.AddError("Gateway delete did not complete", err.Error())
			return
		}
	}

	// If the region is now empty AND it isn't the network's initial region,
	// detach it. We treat the initial region as the one with the oldest
	// instances; that's how the network resource picks it on import.
	net, err := r.client.GetNetwork(ctx, networkID)
	if err != nil {
		// Non-fatal: gateway was deleted; surface as warning.
		resp.Diagnostics.AddWarning("Could not check region after gateway delete", err.Error())
		return
	}
	for i := range net.Regions {
		if net.Regions[i].ID != regionID {
			continue
		}
		if len(net.Regions[i].Instances) == 0 {
			initial := oldestRegion(net.Regions)
			if initial == nil || initial.ID != regionID {
				rmOp, err := r.client.RemoveRegionFromNetwork(ctx, networkID, regionID)
				if err != nil {
					resp.Diagnostics.AddWarning("Failed to remove empty region after gateway delete", err.Error())
					return
				}
				if _, err := r.client.WaitForOperation(ctx, rmOp); err != nil {
					resp.Diagnostics.AddWarning("Empty region delete did not complete", err.Error())
					return
				}
				tflog.Debug(ctx, "removed empty region after gateway delete", map[string]any{"region_id": regionID, "network_id": networkID})
			}
		}
		break
	}
	tflog.Debug(ctx, "deleted harmonysase standard gateway", map[string]any{"id": gatewayID, "network_id": networkID})
}

func (r *gatewayResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	// Accept either:
	//   network_id,gateway_id                            (2 parts)
	//   network_id,gateway_id,harmony_sase_region_id     (3 parts)
	//
	// The 2-part form lets refreshState backfill harmony_sase_region_id with
	// the runtime region ID, which works fine if your config also uses the
	// runtime ID. The 3-part form is the right choice when your config uses
	// a catalogue ID (or a name lookup) — pass the catalogue ID here so it
	// matches what the config will resolve to, avoiding a forced replacement
	// on the first plan after import.
	parts3 := splitImportID(req.ID, 3)
	if parts3 != nil {
		resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("network_id"), parts3[0])...)
		resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), parts3[1])...)
		resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("harmony_sase_region_id"), parts3[2])...)
		return
	}
	parts2 := splitImportID(req.ID, 2)
	if parts2 != nil {
		resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("network_id"), parts2[0])...)
		resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), parts2[1])...)
		return
	}
	resp.Diagnostics.AddError(
		"Invalid import ID",
		"Expected format: network_id,gateway_id  OR  network_id,gateway_id,harmony_sase_region_id",
	)
}

func (r *gatewayResource) refreshState(ctx context.Context, m *gatewayResourceModel, networkID, gatewayID string) error {
	ins, err := r.client.GetInstance(ctx, networkID, gatewayID)
	if err != nil {
		return err
	}
	m.ID = types.StringValue(ins.ID)
	m.NetworkID = types.StringValue(ins.Network)
	m.RegionID = types.StringValue(ins.Region)
	m.InstanceType = types.StringValue(ins.InstanceType)
	m.ImageType = types.StringValue(ins.ImageType)
	m.ImageVersion = types.StringValue(ins.ImageVersion)
	m.IP = types.StringValue(ins.IP)
	m.DNS = types.StringValue(ins.DNS)
	m.TenantID = types.StringValue(ins.TenantID)

	// `harmony_sase_region_id` is the catalogue ID supplied at create time.
	// The API does not return it on read (only the runtime region ID, which
	// we expose separately as `region_id`). So we never overwrite this
	// field once state has a value. On a fresh import where state is null,
	// fall back to the runtime ID so plan sees *something* — users importing
	// a gateway should subsequently set this field to whatever catalogue ID
	// they used at create time (or use the runtime ID and accept that
	// destroy+recreate would land in a different physical region).
	if m.HarmonySaseRegionID.IsNull() || m.HarmonySaseRegionID.IsUnknown() {
		m.HarmonySaseRegionID = types.StringValue(ins.Region)
	}
	if m.Idle.IsNull() || m.Idle.IsUnknown() {
		m.Idle = types.BoolValue(false)
	}
	return nil
}
