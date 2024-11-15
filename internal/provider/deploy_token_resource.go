package provider

import (
	"context"
	"github.com/fly-apps/terraform-provider-fly/graphql"
	"github.com/hashicorp/terraform-plugin-framework-timetypes/timetypes"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"time"
)

type deployTokenResourceModel struct {
	Name      string               `tfsdk:"name"`
	Expiry    timetypes.GoDuration `tfsdk:"expiry"`
	App       types.String         `tfsdk:"app"`
	ID        types.String         `tfsdk:"id"`
	Token     types.String         `tfsdk:"token"`
	CreatedAt types.String         `tfsdk:"created_at"`
	ExpiresAt types.String         `tfsdk:"expires_at"`
}

func (m *deployTokenResourceModel) updateFromGql(token graphql.LimitedAccessToken) {
	m.ID = types.StringValue(token.Id)
	m.Token = types.StringValue(token.Token)
	m.CreatedAt = types.StringValue(token.CreatedAt.Format(time.RFC3339))
	m.ExpiresAt = types.StringValue(token.ExpiresAt.Format(time.RFC3339))
}

type deployTokenResource struct {
	flyResource
}

func newDeployTokenResource() resource.Resource {
	return &deployTokenResource{}
}

func (a deployTokenResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = "fly_deploy_token"
}

func (a deployTokenResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"name": schema.StringAttribute{
				Required:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"expiry": schema.StringAttribute{
				CustomType:    timetypes.GoDurationType{},
				Optional:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"app": schema.StringAttribute{
				Required:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"id": schema.StringAttribute{
				Computed:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"token": schema.StringAttribute{
				Computed:      true,
				Sensitive:     true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"created_at": schema.StringAttribute{
				Computed:      true,
				Sensitive:     true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"expires_at": schema.StringAttribute{
				Computed:      true,
				Sensitive:     true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
		},
	}
}

func (a deployTokenResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan deployTokenResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	app, err := a.apiClient.GetApp(ctx, plan.App.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Failed to get app", err.Error())
	}

	token, err := graphql.CreateLimitedAccessToken(ctx, a.gqlClient, graphql.CreateLimitedAccessTokenInput{
		Expiry:         plan.Expiry.ValueString(),
		Name:           plan.Name,
		OrganizationId: app.Organization.ID,
		Profile:        "deploy",
		ProfileParams: map[string]interface{}{
			"app_id": app.ID,
		},
		OptInThirdParties:  []string{},
		OptOutThirdParties: []string{},
	})
	if err != nil {
		resp.Diagnostics.AddError("failed creating access token", err.Error())
		return
	}

	plan.updateFromGql(token.CreateLimitedAccessToken.LimitedAccessToken.LimitedAccessToken)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (a deployTokenResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	// limitedAccessTokens appears to still be unimplemented on fly.io's server, so this is a no-op for now
	var state deployTokenResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	tokens, err := graphql.LimitedAccessTokens(ctx, a.gqlClient, state.App.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("failed querying access tokens", err.Error())
		return
	}
	for _, t := range tokens.App.LimitedAccessTokens.Nodes {
		if t.Id == state.ID.ValueString() {
			state.updateFromGql(t.LimitedAccessToken)
			resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
			return
		}
	}
	resp.State.RemoveResource(ctx)
}

func (a deployTokenResource) Update(context.Context, resource.UpdateRequest, *resource.UpdateResponse) {
}

func (a deployTokenResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state deployTokenResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	err := a.apiClient.RevokeLimitedAccessToken(ctx, state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("failed deleting access token", err.Error())
		return
	}
}
