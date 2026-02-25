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

var _ resource.Resource = &NetworkResource{}

type NetworkResource struct {
	client *ZeroTierClient
}

type NetworkResourceModel struct {
	ID              types.String `tfsdk:"id"`
	Name            types.String `tfsdk:"name"`
	Description     types.String `tfsdk:"description"`
	Private         types.Bool   `tfsdk:"private"`
	AssignmentPools types.List   `tfsdk:"assignment_pools"`
	Routes          types.List   `tfsdk:"routes"`
}

type AssignmentPoolModel struct {
	Start types.String `tfsdk:"start"`
	End   types.String `tfsdk:"end"`
}

type RouteModel struct {
	Target types.String `tfsdk:"target"`
	Via    types.String `tfsdk:"via"`
}

type ZeroTierNetwork struct {
	ID          string                 `json:"id,omitempty"`
	Name        string                 `json:"name"`
	Description string                 `json:"description,omitempty"`
	Config      *ZeroTierNetworkConfig `json:"config,omitempty"`
}

type ZeroTierNetworkConfig struct {
	Private           bool               `json:"private"`
	IPAssignmentPools []IPAssignmentPool `json:"ipAssignmentPools,omitempty"`
	Routes            []Route            `json:"routes,omitempty"`
	V4AssignMode      *V4AssignMode      `json:"v4AssignMode,omitempty"`
}

type ZeroTierNetworkFlat struct {
	ID                string             `json:"id,omitempty"`
	Name              string             `json:"name"`
	Description       string             `json:"description,omitempty"`
	Private           bool               `json:"private"`
	IPAssignmentPools []IPAssignmentPool `json:"ipAssignmentPools,omitempty"`
	Routes            []Route            `json:"routes,omitempty"`
	V4AssignMode      *V4AssignMode      `json:"v4AssignMode,omitempty"`
}

type IPAssignmentPool struct {
	IPRangeStart string `json:"ipRangeStart"`
	IPRangeEnd   string `json:"ipRangeEnd"`
}

type Route struct {
	Target string `json:"target"`
	Via    string `json:"via,omitempty"`
}

type V4AssignMode struct {
	ZT bool `json:"zt"`
}

func NewNetworkResource() resource.Resource {
	return &NetworkResource{}
}

func (r *NetworkResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_network"
}

func (r *NetworkResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages a ZeroTier network.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:    true,
				Description: "The ZeroTier network ID (16-character hex).",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				Required:    true,
				Description: "Human-readable name of the network.",
			},
			"description": schema.StringAttribute{
				Optional:    true,
				Description: "Short description of the network.",
			},
			"private": schema.BoolAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Whether the network requires authorization to join. Defaults to false.",
			},
			"assignment_pools": schema.ListNestedAttribute{
				Optional:    true,
				Description: "IP assignment pools for auto-assigning addresses to members.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"start": schema.StringAttribute{
							Required:    true,
							Description: "Starting IP address of the pool (e.g., '10.147.20.1').",
						},
						"end": schema.StringAttribute{
							Required:    true,
							Description: "Ending IP address of the pool (e.g., '10.147.20.254').",
						},
					},
				},
			},
			"routes": schema.ListNestedAttribute{
				Optional:    true,
				Description: "Managed routes for the network.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"target": schema.StringAttribute{
							Required:    true,
							Description: "Network to route (e.g., '10.147.20.0/24').",
						},
						"via": schema.StringAttribute{
							Optional:    true,
							Description: "Gateway IP address (optional).",
						},
					},
				},
			},
		},
	}
}

func (r *NetworkResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *NetworkResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan NetworkResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var assignmentPools []IPAssignmentPool
	if !plan.AssignmentPools.IsNull() && !plan.AssignmentPools.IsUnknown() {
		var pools []AssignmentPoolModel
		resp.Diagnostics.Append(plan.AssignmentPools.ElementsAs(ctx, &pools, false)...)
		if resp.Diagnostics.HasError() {
			return
		}
		for _, pool := range pools {
			assignmentPools = append(assignmentPools, IPAssignmentPool{
				IPRangeStart: pool.Start.ValueString(),
				IPRangeEnd:   pool.End.ValueString(),
			})
		}
	}

	var routes []Route
	if !plan.Routes.IsNull() && !plan.Routes.IsUnknown() {
		var routeModels []RouteModel
		resp.Diagnostics.Append(plan.Routes.ElementsAs(ctx, &routeModels, false)...)
		if resp.Diagnostics.HasError() {
			return
		}
		for _, route := range routeModels {
			routes = append(routes, Route{
				Target: route.Target.ValueString(),
				Via:    route.Via.ValueString(),
			})
		}
	}

	var networkID string
	var err error

	if r.client.IsSelfHosted {
		result, createErr := r.client.createNetworkFlat(&ZeroTierNetworkFlat{
			Name:              plan.Name.ValueString(),
			Description:       plan.Description.ValueString(),
			Private:           plan.Private.ValueBool(),
			IPAssignmentPools: assignmentPools,
			Routes:            routes,
			V4AssignMode:      &V4AssignMode{ZT: true},
		})
		err = createErr
		if result != nil {
			networkID = result.ID
		}
	} else {
		result, createErr := r.client.createNetwork(&ZeroTierNetwork{
			Name:        plan.Name.ValueString(),
			Description: plan.Description.ValueString(),
			Config: &ZeroTierNetworkConfig{
				Private:           plan.Private.ValueBool(),
				IPAssignmentPools: assignmentPools,
				Routes:            routes,
				V4AssignMode:      &V4AssignMode{ZT: true},
			},
		})
		err = createErr
		if result != nil {
			networkID = result.ID
		}
	}

	if err != nil {
		resp.Diagnostics.AddError("Error creating network", err.Error())
		return
	}

	plan.ID = types.StringValue(networkID)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *NetworkResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state NetworkResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *NetworkResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan NetworkResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var assignmentPools []IPAssignmentPool
	if !plan.AssignmentPools.IsNull() && !plan.AssignmentPools.IsUnknown() {
		var pools []AssignmentPoolModel
		resp.Diagnostics.Append(plan.AssignmentPools.ElementsAs(ctx, &pools, false)...)
		if resp.Diagnostics.HasError() {
			return
		}
		for _, pool := range pools {
			assignmentPools = append(assignmentPools, IPAssignmentPool{
				IPRangeStart: pool.Start.ValueString(),
				IPRangeEnd:   pool.End.ValueString(),
			})
		}
	}

	var routes []Route
	if !plan.Routes.IsNull() && !plan.Routes.IsUnknown() {
		var routeModels []RouteModel
		resp.Diagnostics.Append(plan.Routes.ElementsAs(ctx, &routeModels, false)...)
		if resp.Diagnostics.HasError() {
			return
		}
		for _, route := range routeModels {
			routes = append(routes, Route{
				Target: route.Target.ValueString(),
				Via:    route.Via.ValueString(),
			})
		}
	}

	var err error

	if r.client.IsSelfHosted {
		_, err = r.client.updateNetworkFlat(plan.ID.ValueString(), &ZeroTierNetworkFlat{
			Name:              plan.Name.ValueString(),
			Description:       plan.Description.ValueString(),
			Private:           plan.Private.ValueBool(),
			IPAssignmentPools: assignmentPools,
			Routes:            routes,
			V4AssignMode:      &V4AssignMode{ZT: true},
		})
	} else {
		_, err = r.client.updateNetwork(plan.ID.ValueString(), &ZeroTierNetwork{
			Name:        plan.Name.ValueString(),
			Description: plan.Description.ValueString(),
			Config: &ZeroTierNetworkConfig{
				Private:           plan.Private.ValueBool(),
				IPAssignmentPools: assignmentPools,
				Routes:            routes,
				V4AssignMode:      &V4AssignMode{ZT: true},
			},
		})
	}

	if err != nil {
		resp.Diagnostics.AddError("Error updating network", err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *NetworkResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state NetworkResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.client.deleteNetwork(state.ID.ValueString()); err != nil {
		resp.Diagnostics.AddError("Error deleting network", err.Error())
	}
}

func (c *ZeroTierClient) createNetwork(n *ZeroTierNetwork) (*ZeroTierNetwork, error) {
	resp, err := c.doRequest("POST", "/network", n)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var result ZeroTierNetwork
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return &result, nil
}

func (c *ZeroTierClient) updateNetwork(id string, n *ZeroTierNetwork) (*ZeroTierNetwork, error) {
	resp, err := c.doRequest("POST", "/network/"+id, n)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var result ZeroTierNetwork
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return &result, nil
}

func (c *ZeroTierClient) createNetworkFlat(n *ZeroTierNetworkFlat) (*ZeroTierNetworkFlat, error) {
	path := "/controller/network/" + c.ControllerID + "______"
	resp, err := c.doRequest("POST", path, n)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var result ZeroTierNetworkFlat
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return &result, nil
}

func (c *ZeroTierClient) updateNetworkFlat(id string, n *ZeroTierNetworkFlat) (*ZeroTierNetworkFlat, error) {
	resp, err := c.doRequest("POST", "/controller/network/"+id, n)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var result ZeroTierNetworkFlat
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return &result, nil
}

func (c *ZeroTierClient) deleteNetwork(id string) error {
	var path string
	if c.IsSelfHosted {
		path = "/controller/network/" + id
	} else {
		path = "/network/" + id
	}

	resp, err := c.doRequest("DELETE", path, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	return nil
}
