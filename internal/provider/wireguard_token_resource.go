package provider

import (
	"context"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/superfly/flyctl/api"
)

type flyWireguardTokenResource struct {
	flyResource
}

func newFlyWireguardTokenResource() resource.Resource {
	return &flyWireguardTokenResource{}
}

type flyWireguardTokenModel struct {
	Token        types.String `tfsdk:"token"`
	Name         types.String `tfsdk:"name"`
	Organization types.String `tfsdk:"organization"`
}

func (f flyWireguardTokenResource) Metadata(_ context.Context, _ resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = "fly_wireguard_token"
}

func (f flyWireguardTokenResource) GetSchema(context.Context) (tfsdk.Schema, diag.Diagnostics) {
	return tfsdk.Schema{
		Attributes: map[string]tfsdk.Attribute{
			"organization": {
				MarkdownDescription: "ID of organization for the token to allow connections into",
				Type:                types.StringType,
				Required:            true,
				PlanModifiers:       []tfsdk.AttributePlanModifier{resource.RequiresReplace()},
			},
			"name": {
				MarkdownDescription: "Optional name for the token",
				Type:                types.StringType,
				Optional:            true,
				PlanModifiers:       []tfsdk.AttributePlanModifier{resource.RequiresReplace()},
			},
			"token": {
				Type:      types.StringType,
				Computed:  true,
				Sensitive: true,
			},
		},
	}, nil
}

func (f flyWireguardTokenResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data flyWireguardTokenModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	name := ""
	if !(data.Name.IsUnknown() || data.Name.IsNull()) {
		name = data.Name.Value
	}
	token, err := f.apiClient.CreateDelegatedWireGuardToken(ctx, &api.Organization{ID: data.Organization.Value}, name)
	if err != nil {
		resp.Diagnostics.AddError("CreateDelegatedWireGuardToken failed", err.Error())
		return
	}

	data.Token = types.String{Value: token.Token}
	resp.Diagnostics.Append(resp.State.Set(ctx, data)...)
}

func (f flyWireguardTokenResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {

}

func (f flyWireguardTokenResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
}

func (f flyWireguardTokenResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data flyWireguardTokenModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	var name, token *string
	if !(data.Name.IsNull() || data.Name.IsUnknown()) {
		name = &data.Name.Value
	} else {
		token = &data.Token.Value
	}
	err := f.apiClient.DeleteDelegatedWireGuardToken(ctx, &api.Organization{ID: data.Organization.Value}, name, token)
	if err != nil {
		resp.Diagnostics.AddError("DeleteDelegatedWireGuardToken failed", err.Error())
		return
	}
}
