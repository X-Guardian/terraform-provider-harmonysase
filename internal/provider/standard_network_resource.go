// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/hashicorp/terraform-plugin-framework-timeouts/resource/timeouts"
	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64default"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	"github.com/X-Guardian/terraform-provider-harmonysase/internal/client"
)

var (
	_ resource.Resource                = &networkResource{}
	_ resource.ResourceWithConfigure   = &networkResource{}
	_ resource.ResourceWithImportState = &networkResource{}
)

const (
	defaultNetworkCreateTimeout = 30 * time.Minute
	defaultNetworkUpdateTimeout = 15 * time.Minute
	defaultNetworkDeleteTimeout = 15 * time.Minute
)

// NewNetworkResource returns a new network resource. Used by provider.go to
// register it.
func NewNetworkResource() resource.Resource { return &networkResource{} }

type networkResource struct {
	client *client.Client
}

// networkResourceModel mirrors the schema.
type networkResourceModel struct {
	ID            types.String        `tfsdk:"id"`
	Name          types.String        `tfsdk:"name"`
	Subnet        types.String        `tfsdk:"subnet"`
	Tags          types.Set           `tfsdk:"tags"`
	InitialRegion *initialRegionModel `tfsdk:"initial_region"`
	Regions       types.Map           `tfsdk:"regions"`
	DNS           types.String        `tfsdk:"dns"`
	AccessType    types.String        `tfsdk:"access_type"`
	TenantID      types.String        `tfsdk:"tenant_id"`
	Timeouts      timeouts.Value      `tfsdk:"timeouts"`
}

type initialRegionModel struct {
	HarmonySaseRegionID types.String `tfsdk:"harmony_sase_region_id"`
	ScaleUnits          types.Int64  `tfsdk:"scale_units"`
	Idle                types.Bool   `tfsdk:"idle"`
	InstanceType        types.String `tfsdk:"instance_type"`
	RegionID            types.String `tfsdk:"region_id"`
	GatewayIDs          types.List   `tfsdk:"gateway_ids"`
}

func (r *networkResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_standard_network"
}

func (r *networkResource) Schema(ctx context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Standard Harmony SASE network. Owns its initial region and the gateway(s) born with it. Additional gateways and regions are managed via `harmonysase_gateway`.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "Network ID assigned by the API.",
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "Network name (5–32 characters).",
				Required:            true,
				Validators: []validator.String{
					stringvalidator.LengthBetween(5, 32),
				},
			},
			"subnet": schema.StringAttribute{
				MarkdownDescription: "CIDR for the network. Defaults to `10.255.0.0/16`. Changing forces replacement.",
				Optional:            true,
				Computed:            true,
				Default:             stringdefault.StaticString("10.255.0.0/16"),
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"tags": schema.SetAttribute{
				MarkdownDescription: "Free-form tags applied to the network.",
				ElementType:         types.StringType,
				Optional:            true,
			},
			"initial_region": schema.SingleNestedAttribute{
				MarkdownDescription: "The first region attached to the network. Required at create time and remains the network's seed region. Use `harmonysase_gateway` for any additional regions or gateways.",
				Required:            true,
				Attributes: map[string]schema.Attribute{
					"harmony_sase_region_id": schema.StringAttribute{
						MarkdownDescription: "ID of the SASE region (from the `/harmony-sase-regions` catalogue). Changing forces replacement.",
						Required:            true,
						PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
					},
					"scale_units": schema.Int64Attribute{
						MarkdownDescription: "Number of gateways in this region. Defaults to 1.",
						Optional:            true,
						Computed:            true,
						Default:             int64default.StaticInt64(1),
						Validators:          []validator.Int64{int64validator.AtLeast(1)},
					},
					"idle": schema.BoolAttribute{
						MarkdownDescription: "Create gateways in idle/disabled state. Defaults to false (gateways are active). Changing forces replacement.",
						Optional:            true,
						Computed:            true,
						Default:             booldefault.StaticBool(false),
						PlanModifiers:       []planmodifier.Bool{boolplanmodifier.RequiresReplace()},
					},
					"instance_type": schema.StringAttribute{
						MarkdownDescription: "Instance type for the gateways in this region. Changing forces replacement.",
						Optional:            true,
						Computed:            true,
						PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace(), stringplanmodifier.UseStateForUnknown()},
					},
					"region_id": schema.StringAttribute{
						MarkdownDescription: "Runtime region ID assigned by the API.",
						Computed:            true,
						PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
					},
					"gateway_ids": schema.ListAttribute{
						MarkdownDescription: "IDs of gateways currently in the initial region, sorted ascending. Used as the deterministic order for scale-down victim selection.",
						ElementType:         types.StringType,
						Computed:            true,
					},
				},
			},
			"regions": schema.MapNestedAttribute{
				MarkdownDescription: "All regions attached to this network, keyed by region name as returned by the API (e.g. `\"London\"`, `\"London 2\"`). Reference these from sibling resources (e.g. `harmonysase_standard_wireguard_tunnel`) to avoid hardcoding runtime IDs.",
				Computed:            true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"id": schema.StringAttribute{
							MarkdownDescription: "Runtime region ID.",
							Computed:            true,
						},
						"gateways": schema.ListNestedAttribute{
							MarkdownDescription: "Gateways currently in this region, ordered by their runtime ID ascending so positions are stable.",
							Computed:            true,
							NestedObject: schema.NestedAttributeObject{
								Attributes: map[string]schema.Attribute{
									"id":            schema.StringAttribute{Computed: true},
									"ip":            schema.StringAttribute{Computed: true},
									"dns":           schema.StringAttribute{Computed: true},
									"instance_type": schema.StringAttribute{Computed: true},
									"image_type":    schema.StringAttribute{Computed: true},
									"image_version": schema.StringAttribute{Computed: true},
									"tenant_id":     schema.StringAttribute{Computed: true},
								},
							},
						},
					},
				},
			},
			"dns": schema.StringAttribute{
				MarkdownDescription: "DNS suffix assigned to the network.",
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"access_type": schema.StringAttribute{
				MarkdownDescription: "`public` or `private`.",
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"tenant_id": schema.StringAttribute{
				Computed:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
		},
		Blocks: map[string]schema.Block{
			"timeouts": timeouts.Block(ctx, timeouts.Opts{Create: true, Update: true, Delete: true}),
		},
	}
}

func (r *networkResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = clientFromProviderData(req.ProviderData, &resp.Diagnostics)
}

func (r *networkResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan networkResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	createTimeout, diags := plan.Timeouts.Create(ctx, defaultNetworkCreateTimeout)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	ctx, cancel := context.WithTimeout(ctx, createTimeout)
	defer cancel()

	var payload client.DeployNetworkPayload
	payload.Network.Name = plan.Name.ValueString()
	if !plan.Subnet.IsNull() && !plan.Subnet.IsUnknown() {
		payload.Network.Subnet = plan.Subnet.ValueString()
	}
	if !plan.Tags.IsNull() && !plan.Tags.IsUnknown() {
		var tags []string
		resp.Diagnostics.Append(plan.Tags.ElementsAs(ctx, &tags, false)...)
		payload.Network.Tags = tags
	}
	payload.Regions = []client.CreateRegionInNetworkPayload{regionPayloadFromModel(plan.InitialRegion)}

	op, err := r.client.CreateNetwork(ctx, payload)
	if err != nil {
		resp.Diagnostics.AddError("Failed to create network", err.Error())
		return
	}
	resultRaw, err := r.client.WaitForOperation(ctx, op)
	if err != nil {
		resp.Diagnostics.AddError("Network create did not complete", err.Error())
		return
	}
	// The status `result` carries the created network object; we want its ID.
	var result struct {
		ID string `json:"id"`
	}
	if len(resultRaw) > 0 {
		_ = jsonUnmarshal(resultRaw, &result)
	}
	if result.ID == "" {
		resp.Diagnostics.AddError(
			"Could not determine network ID after create",
			"The async status payload did not contain an `id` field. This is unexpected; please report it with provider trace logs.",
		)
		return
	}

	if err := r.refreshState(ctx, &plan, result.ID); err != nil {
		resp.Diagnostics.AddError("Failed to read network after create", err.Error())
		return
	}

	tflog.Debug(ctx, "created harmonysase standard network", map[string]any{"id": result.ID})
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *networkResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state networkResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.refreshState(ctx, &state, state.ID.ValueString()); err != nil {
		if client.IsNotFound(err) {
			tflog.Warn(ctx, "harmonysase standard network not found, removing from state", map[string]any{"id": state.ID.ValueString()})
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Failed to read network", err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *networkResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state networkResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	updateTimeout, diags := plan.Timeouts.Update(ctx, defaultNetworkUpdateTimeout)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	ctx, cancel := context.WithTimeout(ctx, updateTimeout)
	defer cancel()

	// 1. Top-level name/tags update.
	if !plan.Name.Equal(state.Name) || !plan.Tags.Equal(state.Tags) {
		var body client.UpdateNetworkPayload
		body.Network.Name = plan.Name.ValueString()
		if !plan.Tags.IsNull() && !plan.Tags.IsUnknown() {
			var tags []string
			resp.Diagnostics.Append(plan.Tags.ElementsAs(ctx, &tags, false)...)
			body.Network.Tags = tags
		}
		if _, err := r.client.UpdateNetwork(ctx, state.ID.ValueString(), body); err != nil {
			resp.Diagnostics.AddError("Failed to update network", err.Error())
			return
		}
	}

	// 2. Initial region scale_units reconciliation.
	if plan.InitialRegion != nil && state.InitialRegion != nil &&
		!plan.InitialRegion.ScaleUnits.Equal(state.InitialRegion.ScaleUnits) {
		desired := plan.InitialRegion.ScaleUnits.ValueInt64()
		current := state.InitialRegion.ScaleUnits.ValueInt64()
		regionID := state.InitialRegion.RegionID.ValueString()
		if regionID == "" {
			resp.Diagnostics.AddError("Cannot reconcile scale_units without runtime region_id", "State is missing initial_region.region_id; try terraform refresh.")
			return
		}
		if err := r.reconcileGatewayCount(ctx, state.ID.ValueString(), regionID, current, desired, state.InitialRegion); err != nil {
			resp.Diagnostics.AddError("Failed to reconcile gateway count", err.Error())
			return
		}
	}

	if err := r.refreshState(ctx, &plan, state.ID.ValueString()); err != nil {
		resp.Diagnostics.AddError("Failed to read network after update", err.Error())
		return
	}
	tflog.Debug(ctx, "updated harmonysase standard network", map[string]any{"id": state.ID.ValueString()})
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *networkResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state networkResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	deleteTimeout, diags := state.Timeouts.Delete(ctx, defaultNetworkDeleteTimeout)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	ctx, cancel := context.WithTimeout(ctx, deleteTimeout)
	defer cancel()

	op, err := r.client.DeleteNetwork(ctx, state.ID.ValueString())
	if err != nil {
		if client.IsConflict(err) {
			resp.Diagnostics.AddError(
				"Network has dependent resources",
				"The API rejected delete with HTTP 409. Destroy any harmonysase_wireguard_tunnel and harmonysase_gateway resources attached to this network first.",
			)
			return
		}
		if client.IsNotFound(err) {
			return
		}
		resp.Diagnostics.AddError("Failed to delete network", err.Error())
		return
	}
	if op.StatusURL != "" {
		if _, err := r.client.WaitForOperation(ctx, op); err != nil {
			resp.Diagnostics.AddError("Network delete did not complete", err.Error())
			return
		}
	}
	tflog.Debug(ctx, "deleted harmonysase standard network", map[string]any{"id": state.ID.ValueString()})
}

func (r *networkResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

// refreshState populates m from the API for the given network ID. It does
// NOT touch m.Timeouts. The InitialRegion is matched by harmony_sase_region_id
// when set in m; otherwise (import path) by oldest createdAt across regions.
func (r *networkResource) refreshState(ctx context.Context, m *networkResourceModel, networkID string) error {
	net, err := r.client.GetNetwork(ctx, networkID)
	if err != nil {
		return err
	}

	m.ID = types.StringValue(net.ID)
	m.Name = types.StringValue(net.Name)
	m.Subnet = types.StringValue(net.Subnet)
	m.DNS = types.StringValue(net.DNS)
	m.AccessType = types.StringValue(net.AccessType)
	m.TenantID = types.StringValue(net.TenantID)

	tagsSet, diags := types.SetValueFrom(ctx, types.StringType, net.Tags)
	if diags.HasError() {
		return fmt.Errorf("encoding tags: %v", diags)
	}
	m.Tags = tagsSet

	// Pick the initial region.
	var picked *client.NetworkRegion
	if m.InitialRegion != nil && !m.InitialRegion.HarmonySaseRegionID.IsNull() && !m.InitialRegion.HarmonySaseRegionID.IsUnknown() {
		want := m.InitialRegion.HarmonySaseRegionID.ValueString()
		for i := range net.Regions {
			if net.Regions[i].Name == want || net.Regions[i].ID == want {
				picked = &net.Regions[i]
				break
			}
		}
	}
	if picked == nil && len(net.Regions) > 0 {
		// Import path: pick region whose oldest instance has the earliest
		// createdAt (or fall back to the first region).
		picked = oldestRegion(net.Regions)
	}
	if picked == nil {
		return fmt.Errorf("network %s has no regions; cannot populate initial_region", net.ID)
	}

	gws := make([]string, 0, len(picked.Instances))
	for _, ins := range picked.Instances {
		gws = append(gws, ins.ID)
	}
	sort.Strings(gws)

	gwList, diags := types.ListValueFrom(ctx, types.StringType, gws)
	if diags.HasError() {
		return fmt.Errorf("encoding gateway_ids: %v", diags)
	}

	if m.InitialRegion == nil {
		m.InitialRegion = &initialRegionModel{}
	}
	m.InitialRegion.RegionID = types.StringValue(picked.ID)
	m.InitialRegion.GatewayIDs = gwList
	m.InitialRegion.ScaleUnits = types.Int64Value(int64(len(picked.Instances)))
	if m.InitialRegion.HarmonySaseRegionID.IsNull() || m.InitialRegion.HarmonySaseRegionID.IsUnknown() {
		// Import: backfill with the runtime region ID. The GET network response
		// does not expose the catalogue ID (the harmonySaseRegionId used at
		// create time), so we use the runtime ID as a stable, unambiguous key
		// that refreshState can match against on subsequent reads. Users who
		// want to recreate from this state should manually replace this value
		// with the catalogue ID from GET /harmony-sase-regions.
		m.InitialRegion.HarmonySaseRegionID = types.StringValue(picked.ID)
	}
	if m.InitialRegion.Idle.IsNull() || m.InitialRegion.Idle.IsUnknown() {
		m.InitialRegion.Idle = types.BoolValue(false)
	}
	if m.InitialRegion.InstanceType.IsNull() || m.InitialRegion.InstanceType.IsUnknown() {
		// Best-effort: take the first instance's type if any.
		if len(picked.Instances) > 0 {
			m.InitialRegion.InstanceType = types.StringValue(picked.Instances[0].InstanceType)
		} else {
			m.InitialRegion.InstanceType = types.StringNull()
		}
	}

	// Populate the computed `regions` map (all regions/gateways, not just initial).
	regionsMap, err := buildRegionsAttr(net.Regions)
	if err != nil {
		return err
	}
	m.Regions = regionsMap

	return nil
}

// reconcileGatewayCount scales an existing region by N gateways. desired and
// current are the user-visible scale_units values; the region must already
// exist on the network. Scale-down picks the highest-id gateways.
func (r *networkResource) reconcileGatewayCount(ctx context.Context, networkID, regionID string, current, desired int64, st *initialRegionModel) error {
	switch {
	case desired == current:
		return nil
	case desired > current:
		// API takes one regionId per POST and adds one gateway. Loop the
		// difference. (If the API later supports a count, swap this out.)
		instanceType := st.InstanceType.ValueString()
		for i := int64(0); i < desired-current; i++ {
			op, err := r.client.AddInstances(ctx, networkID, client.CreateInstancesPayload{
				InstanceType: instanceType,
				RegionIDs:    []string{regionID},
			})
			if err != nil {
				return fmt.Errorf("scale up (add gateway %d/%d): %w", i+1, desired-current, err)
			}
			if _, err := r.client.WaitForOperation(ctx, op); err != nil {
				return fmt.Errorf("scale up (add gateway %d/%d): %w", i+1, desired-current, err)
			}
		}
		return nil
	default: // desired < current
		// Scale-down victims: the tail of gateway_ids (sorted ascending), so
		// "last in, first out" by id.
		var ids []string
		_ = st.GatewayIDs.ElementsAs(ctx, &ids, false)
		sort.Strings(ids)
		toDelete := ids[len(ids)-int(current-desired):]
		op, err := r.client.RemoveInstances(ctx, networkID, client.RemoveInstancesPayload{
			RegionIDs:   []string{regionID},
			InstanceIDs: toDelete,
		})
		if err != nil {
			return fmt.Errorf("scale down: %w", err)
		}
		if _, err := r.client.WaitForOperation(ctx, op); err != nil {
			return fmt.Errorf("scale down: %w", err)
		}
		return nil
	}
}

// regionPayloadFromModel converts the schema model into the API create body.
func regionPayloadFromModel(m *initialRegionModel) client.CreateRegionInNetworkPayload {
	if m == nil {
		return client.CreateRegionInNetworkPayload{}
	}
	out := client.CreateRegionInNetworkPayload{
		HarmonySaseRegionID: m.HarmonySaseRegionID.ValueString(),
	}
	if !m.ScaleUnits.IsNull() && !m.ScaleUnits.IsUnknown() {
		v := m.ScaleUnits.ValueInt64()
		out.ScaleUnits = &v
	}
	if !m.Idle.IsNull() && !m.Idle.IsUnknown() {
		v := m.Idle.ValueBool()
		out.Idle = &v
	}
	if !m.InstanceType.IsNull() && !m.InstanceType.IsUnknown() && m.InstanceType.ValueString() != "" {
		out.InstanceType = m.InstanceType.ValueString()
	}
	return out
}
