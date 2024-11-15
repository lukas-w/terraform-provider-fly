package provider

import (
	"context"
	"errors"
	"fmt"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/setdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
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

func (r redisResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"org": schema.StringAttribute{
				Description:   "Organization slug",
				Required:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"name": schema.StringAttribute{
				Optional:      true,
				Computed:      true,
				Description:   "The name of your Redis database. Leave blank to generate one.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"plan": schema.StringAttribute{
				Required:    true,
				Description: "Plan name. Use `fly redis plans` to list available plans.",
			},
			"primary_region": schema.StringAttribute{
				Description:   "Primary region, can't be changed later",
				Required:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"read_regions": schema.SetAttribute{
				ElementType: types.StringType,
				Optional:    true,
				Computed:    true,
				Default:     setdefault.StaticValue(types.SetValueMust(types.StringType, []attr.Value{})),
			},
			"eviction": schema.BoolAttribute{
				Optional:      true,
				Computed:      true,
				Default:       booldefault.StaticBool(false),
				PlanModifiers: []planmodifier.Bool{boolplanmodifier.RequiresReplace()},
			},
			"public_url": schema.StringAttribute{
				Computed:  true,
				Sensitive: true,
			},
			"password": schema.StringAttribute{
				Sensitive: true,
				Computed:  true,
			},
			"private_ip": schema.StringAttribute{
				Computed: true,
			},
		},
	}
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
	org, err := r.apiClient.GetOrganizationBySlug(ctx, data.Org.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error resolving organization", err.Error())
		return
	}

	// Resolve plan
	planId := r.resolvePlan(ctx, data.Plan.ValueString(), &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	mresp, err := gql.CreateAddOn(ctx, r.gqlClient,
		gql.CreateAddOnInput{
			Type:           gql.AddOnTypeUpstashRedis,
			OrganizationId: org.ID,
			PrimaryRegion:  data.PrimaryRegion.ValueString(),
			Name:           data.Name.ValueString(),
			PlanId:         planId,
			ReadRegions:    readRegions,
			Options: map[string]interface{}{
				"eviction": data.Eviction.ValueBool(),
			},
		},
	)
	if err != nil {
		resp.Diagnostics.AddError("Error creating addon", err.Error())
		return
	}
	data.Name = types.StringValue(mresp.CreateAddOn.AddOn.Name)
	data.PublicUrl = types.StringValue(mresp.CreateAddOn.AddOn.PublicUrl)

	r.get(ctx, &data, &resp.Diagnostics)
	resp.Diagnostics.Append(resp.State.Set(ctx, data)...)
}

func (r redisResource) resolvePlan(ctx context.Context, name string, diags *diag.Diagnostics) string {
	qresp, err := gql.ListAddOnPlans(ctx, r.gqlClient, gql.AddOnTypeUpstashRedis)
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
	if !r.get(ctx, &data, &resp.Diagnostics) {
		resp.State.RemoveResource(ctx)
		return
	}
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(req.State.Set(ctx, data)...)
}

func (r redisResource) get(ctx context.Context, data *addonResourceModel, diags *diag.Diagnostics) bool {
	var d diag.Diagnostics
	qresp, err := gql.GetAddOn(ctx, r.gqlClient, data.Name.ValueString(), string(gql.AddOnTypeUpstashRedis))
	if err != nil {
		var errList gqlerror.List
		if errors.As(err, &errList) {
			for _, err := range errList {
				if strings.Contains(strings.ToLower(err.Message), "could not find") {
					return false
				}
			}
		}
		diags.AddError("Error getting addon", err.Error())
		return true
	}
	data.Name = types.StringValue(qresp.AddOn.Name)
	data.Org = types.StringValue(qresp.AddOn.Organization.Slug)
	data.Plan = types.StringValue(qresp.AddOn.AddOnPlan.DisplayName)
	data.PublicUrl = types.StringValue(qresp.AddOn.PublicUrl)
	data.PrimaryRegion = types.StringValue(qresp.AddOn.PrimaryRegion)
	data.ReadRegions, d = types.SetValueFrom(ctx, types.StringType, qresp.AddOn.ReadRegions)
	diags.Append(d...)
	if diags.HasError() {
		return true
	}

	data.Password = types.StringValue(qresp.AddOn.Password)
	data.PrivateIP = types.StringValue(qresp.AddOn.PrivateIp)
	return true
}

func (r redisResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var state, plan addonResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	planId := r.resolvePlan(ctx, plan.Plan.ValueString(), &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	var readRegions []string
	resp.Diagnostics.Append(plan.ReadRegions.ElementsAs(ctx, readRegions, false)...)
	_, err := gql.UpdateAddOn(ctx, r.gqlClient, state.Name.ValueString(), planId, readRegions, map[string]interface{}{}, map[string]interface{}{})
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

	_, err := gql.DeleteAddOn(ctx, r.gqlClient, data.Name.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error deleting addon", err.Error())
		return
	}
}
