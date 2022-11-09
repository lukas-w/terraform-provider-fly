package provider

import (
	"context"
	"fmt"
	"github.com/fly-apps/terraform-provider-fly/internal/provider/modifiers"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/superfly/flyctl/flypg"
)

type pgUserResourceModel struct {
	App       string `tfsdk:"app"`
	Name      string `tfsdk:"name"`
	Password  string `tfsdk:"password"`
	Superuser bool   `tfsdk:"superuser"`
}

type pgUserResource struct {
	pgResource
}

func NewPgUserResource() resource.Resource {
	return &pgUserResource{}
}

func (u pgUserResource) Metadata(ctx context.Context, request resource.MetadataRequest, response *resource.MetadataResponse) {
	response.TypeName = "fly_pg_user"
}

func (u pgUserResource) GetSchema(context.Context) (tfsdk.Schema, diag.Diagnostics) {
	return tfsdk.Schema{
		Attributes: map[string]tfsdk.Attribute{
			"app": {
				Type:          types.StringType,
				Required:      true,
				PlanModifiers: []tfsdk.AttributePlanModifier{resource.RequiresReplace()},
			},
			"name": {
				Type:          types.StringType,
				Required:      true,
				PlanModifiers: []tfsdk.AttributePlanModifier{resource.RequiresReplace()},
			},
			"password": {
				Type:          types.StringType,
				Required:      true,
				Sensitive:     true,
				PlanModifiers: []tfsdk.AttributePlanModifier{resource.RequiresReplace()},
			},
			"superuser": {
				Type:          types.BoolType,
				Optional:      false,
				Computed:      true,
				PlanModifiers: []tfsdk.AttributePlanModifier{resource.RequiresReplace(), modifiers.Default(types.Bool{Value: false})},
			},
		},
	}, nil
}

func (u pgUserResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan pgUserResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	client := u.getPgClient(ctx, plan.App, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	r := client.Post("/commands/users/create").SetBody(
		flypg.CreateUserRequest{
			Username:  plan.Name,
			Password:  plan.Password,
			Superuser: plan.Superuser,
		},
	).Do(ctx)
	if r.Err != nil {
		resp.Diagnostics.AddError("API call failed", r.Err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (u pgUserResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state pgUserResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	client := u.getPgClient(ctx, state.App, &resp.Diagnostics)
	var result flypg.FindUserResponse
	r := client.Get(fmt.Sprintf("/commands/users/%s", state.Name)).SetResult(&result).Do(ctx)
	if r.StatusCode == 404 {
		resp.State.RemoveResource(ctx)
		return
	}
	if r.Err != nil {
		resp.Diagnostics.AddError("API call failed", r.Err.Error())
		return
	}

	state.Name = result.Result.Username
	state.Superuser = result.Result.Superuser
	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}

func (u pgUserResource) Update(ctx context.Context, request resource.UpdateRequest, response *resource.UpdateResponse) {
}

func (u pgUserResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var plan pgUserResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	client := u.getPgClient(ctx, plan.App, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	r := client.Delete("/commands/users/delete").SetBody(
		flypg.DeleteUserRequest{
			Username: plan.Name,
		},
	).Do(ctx)
	if r.Err != nil {
		resp.Diagnostics.AddError("API call failed", r.Err.Error())
		return
	}
}
