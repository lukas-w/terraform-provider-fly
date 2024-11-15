package provider

import (
	"context"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/superfly/fly-go"
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

func (f flyWireguardTokenResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"organization": schema.StringAttribute{
				MarkdownDescription: "ID of organization for the token to allow connections into",
				Required:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "Optional name for the token",
				Optional:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"token": schema.StringAttribute{
				Computed:  true,
				Sensitive: true,
			},
		},
	}
}

func (f flyWireguardTokenResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data flyWireguardTokenModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	name := ""
	if !(data.Name.IsUnknown() || data.Name.IsNull()) {
		name = data.Name.ValueString()
	}
	token, err := f.apiClient.CreateDelegatedWireGuardToken(ctx, &fly.Organization{ID: data.Organization.ValueString()}, name)
	if err != nil {
		resp.Diagnostics.AddError("CreateDelegatedWireGuardToken failed", err.Error())
		return
	}

	data.Token = types.StringValue(token.Token)
	resp.Diagnostics.Append(resp.State.Set(ctx, data)...)
}

func (f flyWireguardTokenResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {

}

func (f flyWireguardTokenResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
}

func (f flyWireguardTokenResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data flyWireguardTokenModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	var name, token string
	if !(data.Name.IsNull() || data.Name.IsUnknown()) {
		name = data.Name.ValueString()
	} else {
		token = data.Token.ValueString()
	}
	err := f.apiClient.DeleteDelegatedWireGuardToken(ctx, &fly.Organization{ID: data.Organization.ValueString()}, &name, &token)
	if err != nil {
		resp.Diagnostics.AddError("DeleteDelegatedWireGuardToken failed", err.Error())
		return
	}
}
