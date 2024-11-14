package provider

import (
	"context"
	"github.com/fly-apps/terraform-provider-fly/graphql"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"time"
)

type deployTokenResourceModel struct {
	Name      string            `tfsdk:"name"`
	Expiry    DurationAttrValue `tfsdk:"expiry"`
	App       types.String      `tfsdk:"app"`
	ID        types.String      `tfsdk:"id"`
	Token     types.String      `tfsdk:"token"`
	CreatedAt types.String      `tfsdk:"created_at"`
	ExpiresAt types.String      `tfsdk:"expires_at"`
}

func (m *deployTokenResourceModel) updateFromGql(token graphql.LimitedAccessToken) {
	m.ID = types.String{Value: token.Id}
	m.Token = types.String{Value: token.Token}
	m.CreatedAt = types.String{Value: token.CreatedAt.Format(time.RFC3339)}
	m.ExpiresAt = types.String{Value: token.ExpiresAt.Format(time.RFC3339)}
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

func (a deployTokenResource) GetSchema(ctx context.Context) (tfsdk.Schema, diag.Diagnostics) {
	return tfsdk.Schema{
		Attributes: map[string]tfsdk.Attribute{
			"name": {
				Type:          types.StringType,
				Required:      true,
				PlanModifiers: []tfsdk.AttributePlanModifier{resource.RequiresReplace()},
			},
			"expiry": {
				Type:          DurationType{},
				Optional:      true,
				Computed:      true,
				PlanModifiers: []tfsdk.AttributePlanModifier{resource.RequiresReplace()},
			},
			"app": {
				Type:          types.StringType,
				Required:      true,
				PlanModifiers: []tfsdk.AttributePlanModifier{resource.RequiresReplace()},
			},
			"id": {
				Type:          types.StringType,
				Computed:      true,
				PlanModifiers: []tfsdk.AttributePlanModifier{resource.UseStateForUnknown()},
			},
			"token": {
				Type:          types.StringType,
				Computed:      true,
				Sensitive:     true,
				PlanModifiers: []tfsdk.AttributePlanModifier{resource.UseStateForUnknown()},
			},
			"created_at": {
				Type:          types.StringType,
				Computed:      true,
				Sensitive:     true,
				PlanModifiers: []tfsdk.AttributePlanModifier{resource.UseStateForUnknown()},
			},
			"expires_at": {
				Type:          types.StringType,
				Computed:      true,
				Sensitive:     true,
				PlanModifiers: []tfsdk.AttributePlanModifier{resource.UseStateForUnknown()},
			},
		},
	}, nil
}

func (a deployTokenResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan deployTokenResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	app, err := a.apiClient.GetApp(ctx, plan.App.Value)
	if err != nil {
		resp.Diagnostics.AddError("Failed to get app", err.Error())
	}

	token, err := graphql.CreateLimitedAccessToken(ctx, a.gqlClient, graphql.CreateLimitedAccessTokenInput{
		Expiry:         plan.Expiry.String(),
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
	tokens, err := graphql.LimitedAccessTokens(ctx, a.gqlClient, state.App.Value)
	if err != nil {
		resp.Diagnostics.AddError("failed querying access tokens", err.Error())
		return
	}
	for _, t := range tokens.App.LimitedAccessTokens.Nodes {
		if t.Id == state.ID.Value {
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

	err := a.apiClient.RevokeLimitedAccessToken(ctx, state.ID.Value)
	if err != nil {
		resp.Diagnostics.AddError("failed deleting access token", err.Error())
		return
	}
}
