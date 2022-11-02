package provider

import (
	"context"
	"errors"
	"fmt"
	"github.com/fly-apps/terraform-provider-fly/internal/provider/modifiers"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/superfly/flyctl/gql"
	"github.com/vektah/gqlparser/v2/gqlerror"
	"strings"
)

type addonResourceModel struct {
	Name          types.String `tfsdk:"name"`
	Org           types.String `tfsdk:"org"`
	Plan          types.String `tfsdk:"plan"`
	PrimaryRegion types.String `tfsdk:"primary_region"`
	ReadRegions   types.Set    `tfsdk:"read_regions"`
	Eviction      types.Bool   `tfsdk:"eviction"`
	PublicUrl     types.String `tfsdk:"public_url"`
	Password      types.String `tfsdk:"password"`
	PrivateIP     types.String `tfsdk:"private_ip"`
}

type redisResource struct{ flyResource }

func newRedisResource() resource.Resource {
	return &redisResource{}
}

func (r redisResource) Metadata(ctx context.Context, request resource.MetadataRequest, response *resource.MetadataResponse) {
	response.TypeName = "fly_redis"
}

func (r redisResource) GetSchema(ctx context.Context) (tfsdk.Schema, diag.Diagnostics) {
	return tfsdk.Schema{
		Attributes: map[string]tfsdk.Attribute{
			"org": {
				Description:   "Organization slug",
				Type:          types.StringType,
				Required:      true,
				PlanModifiers: []tfsdk.AttributePlanModifier{resource.RequiresReplace()},
			},
			"name": {
				Type:          types.StringType,
				Optional:      true,
				Computed:      true,
				Description:   "The name of your Redis database. Leave blank to generate one.",
				PlanModifiers: []tfsdk.AttributePlanModifier{resource.RequiresReplace()},
			},
			"plan": {
				Type:        types.StringType,
				Required:    true,
				Description: "Plan name. Use `fly redis plans` to list available plans.",
			},
			"primary_region": {
				Description:   "Primary region, can't be changed later",
				Type:          types.StringType,
				Required:      true,
				PlanModifiers: []tfsdk.AttributePlanModifier{resource.RequiresReplace()},
			},
			"read_regions": {
				Type:     types.SetType{ElemType: types.StringType},
				Optional: true,
				Computed: true,
				PlanModifiers: []tfsdk.AttributePlanModifier{
					modifiers.Default(types.Set{ElemType: types.StringType, Elems: []attr.Value{}}),
				},
			},
			"eviction": {
				Type:     types.BoolType,
				Optional: true,
				Computed: true,
				PlanModifiers: []tfsdk.AttributePlanModifier{
					resource.RequiresReplace(),
					modifiers.Default(types.Bool{Value: false}),
				},
			},
			"public_url": {
				Type:      types.StringType,
				Computed:  true,
				Sensitive: true,
			},
			"password": {
				Type:      types.StringType,
				Sensitive: true,
				Computed:  true,
			},
			"private_ip": {
				Type:     types.StringType,
				Computed: true,
			},
		},
	}, nil
}

func (r redisResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data addonResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var readRegions []string
	resp.Diagnostics.Append(data.ReadRegions.ElementsAs(ctx, &readRegions, false)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Resolve organization
	org, err := r.apiClient.GetOrganizationBySlug(ctx, data.Org.Value)
	if err != nil {
		resp.Diagnostics.AddError("Error resolving organization", err.Error())
		return
	}

	// Resolve plan
	planId := r.resolvePlan(ctx, data.Plan.Value, resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	mresp, err := gql.CreateAddOn(ctx, r.gqlClient,
		org.ID,
		data.PrimaryRegion.Value,
		data.Name.Value,
		planId,
		readRegions,
		map[string]interface{}{
			"eviction": data.Eviction.Value,
		},
	)
	if err != nil {
		resp.Diagnostics.AddError("Error creating addon", err.Error())
		return
	}
	data.Name = types.String{Value: mresp.CreateAddOn.AddOn.Name}
	data.PublicUrl = types.String{Value: mresp.CreateAddOn.AddOn.PublicUrl}

	r.get(ctx, &data, resp.Diagnostics)
	resp.Diagnostics.Append(resp.State.Set(ctx, data)...)
}

func (r redisResource) resolvePlan(ctx context.Context, name string, diags diag.Diagnostics) string {
	qresp, err := gql.ListAddOnPlans(ctx, r.gqlClient)
	if err != nil {
		diags.AddError("Error getting addon plans", err.Error())
		return ""
	}
	for _, p := range qresp.AddOnPlans.Nodes {
		if p.DisplayName == name {
			return p.Id
		}
	}
	diags.AddError("Can't resolve plan", fmt.Sprintf("No plan with name %s", name))
	return ""
}

func (r redisResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data addonResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if !r.get(ctx, &data, resp.Diagnostics) {
		resp.State.RemoveResource(ctx)
		return
	}
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(req.State.Set(ctx, data)...)
}

func (r redisResource) get(ctx context.Context, data *addonResourceModel, diag diag.Diagnostics) bool {
	qresp, err := gql.GetAddOn(ctx, r.gqlClient, data.Name.Value)
	if err != nil {
		var errList gqlerror.List
		if errors.As(err, &errList) {
			for _, err := range errList {
				if strings.Contains(strings.ToLower(err.Message), "could not find") {
					return false
				}
			}
		}
		diag.AddError("Error getting addon", err.Error())
		return true
	}
	data.Name = types.String{Value: qresp.AddOn.Name}
	data.Org = types.String{Value: qresp.AddOn.Organization.Slug}
	data.Plan = types.String{Value: qresp.AddOn.AddOnPlan.DisplayName}
	data.PublicUrl = types.String{Value: qresp.AddOn.PublicUrl}
	data.PrimaryRegion = types.String{Value: qresp.AddOn.PrimaryRegion}
	data.ReadRegions = types.Set{ElemType: types.StringType, Elems: make([]attr.Value, len(qresp.AddOn.ReadRegions))}
	for i, region := range qresp.AddOn.ReadRegions {
		data.ReadRegions.Elems[i] = types.String{Value: region}
	}
	data.Password = types.String{Value: qresp.AddOn.Password}
	data.PrivateIP = types.String{Value: qresp.AddOn.PrivateIp}
	return true
}

func (r redisResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var state, plan addonResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	planId := r.resolvePlan(ctx, plan.Plan.Value, resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	var readRegions []string
	resp.Diagnostics.Append(plan.ReadRegions.ElementsAs(ctx, readRegions, false)...)
	_, err := gql.UpdateAddOn(ctx, r.gqlClient, state.Name.Value, planId, readRegions)
	if err != nil {
		resp.Diagnostics.AddError("Error updating addon", err.Error())
		return
	}
	state.Plan = plan.Plan
	state.ReadRegions = plan.ReadRegions
	resp.Diagnostics.Append(req.State.Set(ctx, state)...)
}

func (r redisResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data addonResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	_, err := gql.DeleteAddOn(ctx, r.gqlClient, data.Name.Value)
	if err != nil {
		resp.Diagnostics.AddError("Error deleting addon", err.Error())
		return
	}
}
