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

type accessTokenResourceModel struct {
	Name      string       `tfsdk:"name"`
	ID        types.String `tfsdk:"id"`
	Token     types.String `tfsdk:"token"`
	CreatedAt types.String `tfsdk:"created-at"`
}

type accessTokenResource struct {
	flyResource
}

func (a accessTokenResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = "fly_access_token"
}

func (a accessTokenResource) GetSchema(ctx context.Context) (tfsdk.Schema, diag.Diagnostics) {
	return tfsdk.Schema{
		Attributes: map[string]tfsdk.Attribute{
			"name": {
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
		},
	}, nil
}

func (a accessTokenResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan accessTokenResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	token, err := graphql.CreateAccessToken(ctx, a.gqlClient, plan.Name)
	if err != nil {
		resp.Diagnostics.AddError("failed creating access token", err.Error())
		return
	}
	plan.ID = types.String{Value: token.CreateAccessToken.AccessToken.Id}
	plan.Token = types.String{Value: token.CreateAccessToken.AccessToken.Token}
	plan.CreatedAt = types.String{Value: token.CreateAccessToken.AccessToken.CreatedAt.Format(time.RFC3339)}
	resp.Diagnostics.Append(req.Plan.Set(ctx, &plan)...)
}

func (a accessTokenResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state accessTokenResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	tokens, err := graphql.GetAccessTokens(ctx, a.gqlClient)
	if err != nil {
		resp.Diagnostics.AddError("failed querying access tokens", err.Error())
		return
	}
	for _, t := range tokens.AccessTokens.Nodes {
		if t.Id == state.ID.Value {
			state.ID = types.String{Value: t.Id}
			state.Token = types.String{Value: t.Token}
			state.CreatedAt = types.String{Value: t.CreatedAt.Format(time.RFC3339)}
			return
		}
	}
	resp.State.RemoveResource(ctx)
}

func (a accessTokenResource) Update(context.Context, resource.UpdateRequest, *resource.UpdateResponse) {
}

func (a accessTokenResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state accessTokenResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	_, err := graphql.DeleteAccessToken(ctx, a.gqlClient, state.ID.Value)
	if err != nil {
		resp.Diagnostics.AddError("failed deleting access token", err.Error())
		return
	}
}
