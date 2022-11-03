package provider

import (
	"context"
	"errors"
	"github.com/fly-apps/terraform-provider-fly/graphql"
	"github.com/fly-apps/terraform-provider-fly/internal/provider/modifiers"
	"github.com/fly-apps/terraform-provider-fly/internal/utils"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/superfly/flyctl/api"
	"github.com/vektah/gqlparser/v2/gqlerror"
)

var _ resource.ResourceWithConfigure = &flyAppResource{}
var _ resource.ResourceWithImportState = &flyAppResource{}

type flyAppResourceData struct {
	Name     types.String `tfsdk:"name"`
	Org      types.String `tfsdk:"org"`
	OrgId    types.String `tfsdk:"orgid"`
	AppUrl   types.String `tfsdk:"appurl"`
	Id       types.String `tfsdk:"id"`
	Machines types.Bool   `tfsdk:"machines"`
}

func appDataFromApi(f api.App) flyAppResourceData {
	return flyAppResourceData{
		Name:     types.String{Value: f.Name},
		Org:      types.String{Value: f.Organization.Slug},
		OrgId:    types.String{Value: f.Organization.ID},
		AppUrl:   types.String{Value: f.AppURL},
		Id:       types.String{Value: f.ID},
		Machines: types.Bool{Value: f.PlatformVersion == string(graphql.PlatformVersionEnumMachines)},
	}
}

func (r flyAppResource) Metadata(_ context.Context, _ resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = "fly_app"
}

func (r flyAppResource) GetSchema(context.Context) (tfsdk.Schema, diag.Diagnostics) {
	return tfsdk.Schema{
		MarkdownDescription: "Fly app resource",
		Attributes: map[string]tfsdk.Attribute{
			"name": {
				MarkdownDescription: "Name of application",
				Required:            true,
				Type:                types.StringType,
				PlanModifiers:       []tfsdk.AttributePlanModifier{resource.RequiresReplace()},
			},
			"org": {
				Computed:            true,
				Optional:            true,
				MarkdownDescription: "Optional org slug to operate upon",
				Type:                types.StringType,
				PlanModifiers:       []tfsdk.AttributePlanModifier{resource.RequiresReplace()},
			},
			"machines": {
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Use the machines platform",
				Type:                types.BoolType,
				PlanModifiers: []tfsdk.AttributePlanModifier{
					resource.RequiresReplace(),
					modifiers.Default(types.Bool{Value: false}),
				},
			},
			"orgid": {
				Computed:            true,
				MarkdownDescription: "readonly orgid",
				Type:                types.StringType,
			},
			"id": {
				Computed:            true,
				MarkdownDescription: "readonly app id",
				Type:                types.StringType,
			},
			"appurl": {
				Computed:            true,
				MarkdownDescription: "readonly appUrl",
				Type:                types.StringType,
			},
		},
	}, nil
}

func newAppResource() resource.Resource {
	return &flyAppResource{}
}

type flyAppResource struct {
	flyResource
}

func (r flyAppResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data flyAppResourceData

	diags := req.Plan.Get(ctx, &data)
	resp.Diagnostics.Append(diags...)

	if resp.Diagnostics.HasError() {
		return
	}

	org := r.resolveOrganization(ctx, data.Org, resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	data.Org.Value = org.Name
	data.OrgId.Value = org.ID

	app, err := r.apiClient.CreateApp(ctx, api.CreateAppInput{
		Name:           data.Name.Value,
		OrganizationID: data.OrgId.Value,
		Machines:       data.Machines.Value,
	})
	if err != nil {
		resp.Diagnostics.AddError("Create app failed", err.Error())
		return
	}

	data = appDataFromApi(*app)

	diags = resp.State.Set(ctx, &data)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}

func (r flyAppResource) resolveOrganization(ctx context.Context, slug types.String, diags diag.Diagnostics) (org *api.Organization) {
	var err error
	if slug.IsUnknown() || slug.IsNull() {
		org, err = utils.GetDefaultOrg(ctx, r.apiClient)
	} else {
		org, err = r.apiClient.GetOrganizationBySlug(ctx, slug.Value)
	}
	if err != nil {
		diags.AddError("Could not resolve organization", err.Error())
	}
	return
}

func (r flyAppResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state flyAppResourceData

	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)

	if resp.Diagnostics.HasError() {
		return
	}

	app, err := r.apiClient.GetApp(ctx, state.Name.Value)
	var errList gqlerror.List
	if errors.As(err, &errList) {
		for _, err := range errList {
			if err.Message == "Could not resolve " {
				resp.State.RemoveResource(ctx)
				return
			} else {
				resp.Diagnostics.AddError(err.Message, err.Path.String())
			}
		}
	} else if err != nil {
		resp.Diagnostics.AddError("Read: query failed", err.Error())
	}
	if resp.Diagnostics.HasError() {
		return
	}

	data := appDataFromApi(*app)

	diags = resp.State.Set(ctx, &data)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}

func (r flyAppResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state flyAppResourceData
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if plan.Org != state.Org {
		org := r.resolveOrganization(ctx, plan.Org, resp.Diagnostics)
		if resp.Diagnostics.HasError() {
			return
		}
		app, err := r.apiClient.MoveApp(ctx, state.Name.Value, org.ID)
		if err != nil {
			resp.Diagnostics.AddError("Error moving app", err.Error())
			return
		}
		state.Org = types.String{Value: app.Organization.Slug}
		state.Id = types.String{Value: app.Organization.ID}
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}

func (r flyAppResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data flyAppResourceData

	diags := req.State.Get(ctx, &data)
	resp.Diagnostics.Append(diags...)

	err := r.apiClient.DeleteApp(ctx, data.Name.Value)
	var errList gqlerror.List
	if errors.As(err, &errList) {
		for _, err := range errList {
			resp.Diagnostics.AddError(err.Message, err.Path.String())
		}
	} else if err != nil {
		resp.Diagnostics.AddError("Delete app failed", err.Error())
	}
}

func (r flyAppResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("name"), req, resp)
}
