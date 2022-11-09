package provider

import (
	"context"
	"fmt"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/superfly/flyctl/flypg"
)

type pgDbResourceModel struct {
	App  string `tfsdk:"app"`
	Name string `tfsdk:"name"`
}

type pgDbResource struct {
	pgResource
}

func NewPgDbResource() resource.Resource {
	return &pgDbResource{}
}

func (d pgDbResource) Metadata(ctx context.Context, request resource.MetadataRequest, response *resource.MetadataResponse) {
	response.TypeName = "fly_pg_db"
}

func (d pgDbResource) GetSchema(context.Context) (tfsdk.Schema, diag.Diagnostics) {
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
		},
	}, nil
}

func (d pgDbResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan pgDbResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	client := d.getPgClient(ctx, plan.App, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	r := client.Post("/commands/databases/create").SetBody(
		flypg.CreateDatabaseRequest{
			Name: plan.Name,
		},
	).Do(ctx)
	if r.Err != nil {
		resp.Diagnostics.AddError("API call failed", r.Err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (d pgDbResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state pgDbResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	client := d.getPgClient(ctx, state.App, &resp.Diagnostics)
	var result flypg.FindDatabaseResponse
	r := client.Get(fmt.Sprintf("/commands/databases/%s", state.Name)).SetResult(&result).Do(ctx)
	if r.StatusCode == 404 {
		resp.State.RemoveResource(ctx)
		return
	}
	if r.Err != nil {
		resp.Diagnostics.AddError("API call failed", r.Err.Error())
		return
	}

	state.Name = result.Result.Name
	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}

func (d pgDbResource) Update(ctx context.Context, request resource.UpdateRequest, response *resource.UpdateResponse) {
}

func (d pgDbResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var plan pgDbResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	client := d.getPgClient(ctx, plan.App, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	r := client.Delete("/commands/databases/delete").SetBody(
		flypg.DeleteDatabaseRequest{
			Name: plan.Name,
		},
	).Do(ctx)
	if r.Err != nil {
		resp.Diagnostics.AddError("API call failed", r.Err.Error())
		return
	}
}
