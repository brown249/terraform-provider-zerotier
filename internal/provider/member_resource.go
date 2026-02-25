package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ resource.Resource = &MemberResource{}

type MemberResource struct {
	client *ZeroTierClient
}

type MemberResourceModel struct {
	ID              types.String `tfsdk:"id"`
	NetworkID       types.String `tfsdk:"network_id"`
	NodeID          types.String `tfsdk:"node_id"`
	Name            types.String `tfsdk:"name"`
	Description     types.String `tfsdk:"description"`
	Authorized      types.Bool   `tfsdk:"authorized"`
	IPAssignments   types.List   `tfsdk:"ip_assignments"`
	NoAutoAssignIPs types.Bool   `tfsdk:"no_auto_assign_ips"`
}

type ZeroTierMember struct {
	ID          string                `json:"id,omitempty"`
	NetworkID   string                `json:"networkId,omitempty"`
	NodeID      string                `json:"nodeId,omitempty"`
	Name        string                `json:"name,omitempty"`
	Description string                `json:"description,omitempty"`
	Config      *ZeroTierMemberConfig `json:"config,omitempty"`
}

type ZeroTierMemberConfig struct {
	Authorized      bool     `json:"authorized"`
	IPAssignments   []string `json:"ipAssignments,omitempty"`
	NoAutoAssignIPs bool     `json:"noAutoAssignIps,omitempty"`
}

func NewMemberResource() resource.Resource {
	return &MemberResource{}
}

func (r *MemberResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_member"
}

func (r *MemberResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages a ZeroTier network member (device authorization).",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:    true,
				Description: "Member identifier (network_id-node_id).",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"network_id": schema.StringAttribute{
				Required:    true,
				Description: "The ZeroTier network ID this member belongs to.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"node_id": schema.StringAttribute{
				Required:    true,
				Description: "The ZeroTier node ID (10-digit hex address) of the device.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"name": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Human-readable name for this member.",
			},
			"description": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Description of the member.",
			},
			"authorized": schema.BoolAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Whether this member is authorized to communicate on the network.",
			},
			"ip_assignments": schema.ListAttribute{
				ElementType: types.StringType,
				Optional:    true,
				Computed:    true,
				Description: "List of IP addresses assigned to this member (e.g., ['10.147.20.10']).",
			},
			"no_auto_assign_ips": schema.BoolAttribute{
				Optional:    true,
				Computed:    true,
				Description: "If true, disable automatic IP assignment for this member.",
			},
		},
	}
}

func (r *MemberResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(*ZeroTierClient)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *ZeroTierClient, got: %T", req.ProviderData),
		)
		return
	}

	r.client = client
}

func (r *MemberResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan MemberResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var ipAssignments []string
	if !plan.IPAssignments.IsNull() && !plan.IPAssignments.IsUnknown() {
		resp.Diagnostics.Append(plan.IPAssignments.ElementsAs(ctx, &ipAssignments, false)...)
		if resp.Diagnostics.HasError() {
			return
		}
	}

	member := &ZeroTierMember{
		Name:        plan.Name.ValueString(),
		Description: plan.Description.ValueString(),
		Config: &ZeroTierMemberConfig{
			Authorized:      plan.Authorized.ValueBool(),
			IPAssignments:   ipAssignments,
			NoAutoAssignIPs: plan.NoAutoAssignIPs.ValueBool(),
		},
	}

	created, err := r.client.updateMember(
		plan.NetworkID.ValueString(),
		plan.NodeID.ValueString(),
		member,
	)
	if err != nil {
		resp.Diagnostics.AddError("Error authorizing member", err.Error())
		return
	}

	plan.ID = types.StringValue(fmt.Sprintf("%s-%s",
		plan.NetworkID.ValueString(),
		plan.NodeID.ValueString(),
	))

	if created.Name != "" {
		plan.Name = types.StringValue(created.Name)
	}
	if created.Description != "" {
		plan.Description = types.StringValue(created.Description)
	}

	if created.Config != nil {
		plan.Authorized = types.BoolValue(created.Config.Authorized)
		plan.NoAutoAssignIPs = types.BoolValue(created.Config.NoAutoAssignIPs)

		ips := created.Config.IPAssignments
		if ips == nil {
			ips = []string{}
		}
		ipList, diags := types.ListValueFrom(ctx, types.StringType, ips)
		resp.Diagnostics.Append(diags...)
		plan.IPAssignments = ipList
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *MemberResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state MemberResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	member, err := r.client.getMember(
		state.NetworkID.ValueString(),
		state.NodeID.ValueString(),
	)
	if err != nil {
		resp.Diagnostics.AddError("Error reading member", err.Error())
		return
	}

	if member.Name != "" {
		state.Name = types.StringValue(member.Name)
	}
	if member.Description != "" {
		state.Description = types.StringValue(member.Description)
	}

	if member.Config != nil {
		state.Authorized = types.BoolValue(member.Config.Authorized)
		state.NoAutoAssignIPs = types.BoolValue(member.Config.NoAutoAssignIPs)

		ips := member.Config.IPAssignments
		if ips == nil {
			ips = []string{}
		}
		ipList, diags := types.ListValueFrom(ctx, types.StringType, ips)
		resp.Diagnostics.Append(diags...)
		state.IPAssignments = ipList
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *MemberResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan MemberResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var ipAssignments []string
	if !plan.IPAssignments.IsNull() && !plan.IPAssignments.IsUnknown() {
		resp.Diagnostics.Append(plan.IPAssignments.ElementsAs(ctx, &ipAssignments, false)...)
		if resp.Diagnostics.HasError() {
			return
		}
	}

	member := &ZeroTierMember{
		Name:        plan.Name.ValueString(),
		Description: plan.Description.ValueString(),
		Config: &ZeroTierMemberConfig{
			Authorized:      plan.Authorized.ValueBool(),
			IPAssignments:   ipAssignments,
			NoAutoAssignIPs: plan.NoAutoAssignIPs.ValueBool(),
		},
	}

	updated, err := r.client.updateMember(
		plan.NetworkID.ValueString(),
		plan.NodeID.ValueString(),
		member,
	)
	if err != nil {
		resp.Diagnostics.AddError("Error updating member", err.Error())
		return
	}

	if updated.Name != "" {
		plan.Name = types.StringValue(updated.Name)
	}
	if updated.Description != "" {
		plan.Description = types.StringValue(updated.Description)
	}

	if updated.Config != nil {
		plan.Authorized = types.BoolValue(updated.Config.Authorized)
		plan.NoAutoAssignIPs = types.BoolValue(updated.Config.NoAutoAssignIPs)

		ips := updated.Config.IPAssignments
		if ips == nil {
			ips = []string{}
		}
		ipList, diags := types.ListValueFrom(ctx, types.StringType, ips)
		resp.Diagnostics.Append(diags...)
		plan.IPAssignments = ipList
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *MemberResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state MemberResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	member := &ZeroTierMember{
		Config: &ZeroTierMemberConfig{
			Authorized: false,
		},
	}

	if _, err := r.client.updateMember(
		state.NetworkID.ValueString(),
		state.NodeID.ValueString(),
		member,
	); err != nil {
		resp.Diagnostics.AddError("Error deauthorizing member", err.Error())
	}
}

func (c *ZeroTierClient) getMember(networkID, nodeID string) (*ZeroTierMember, error) {
	var path string
	if c.IsSelfHosted {
		path = fmt.Sprintf("/controller/network/%s/member/%s", networkID, nodeID)
	} else {
		path = fmt.Sprintf("/network/%s/member/%s", networkID, nodeID)
	}

	resp, err := c.doRequest("GET", path, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var result ZeroTierMember
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return &result, nil
}

func (c *ZeroTierClient) updateMember(networkID, nodeID string, m *ZeroTierMember) (*ZeroTierMember, error) {
	var path string
	if c.IsSelfHosted {
		path = fmt.Sprintf("/controller/network/%s/member/%s", networkID, nodeID)
	} else {
		path = fmt.Sprintf("/network/%s/member/%s", networkID, nodeID)
	}

	resp, err := c.doRequest("POST", path, m)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var result ZeroTierMember
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return &result, nil
}
