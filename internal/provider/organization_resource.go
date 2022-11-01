package provider

import (
	"context"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/superfly/flyctl/api"
)

type organizationResourceModel struct {
	Name types.String `tfsdk:"name"`
	ID   types.String `tfsdk:"id"`
	Slug types.String `tfsdk:"slug"`
}
type organizationResource struct {
	flyResource
}

func (m *organizationResourceModel) fromApiOrganization(org api.Organization) {
	m.Name = types.String{Value: org.Name}
	m.ID = types.String{Value: org.ID}
	m.Slug = types.String{Value: org.Slug}
}

func (r organizationResource) Metadata(_ context.Context, _ resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = "fly_organization"
}

func (r organizationResource) GetSchema(context.Context) (tfsdk.Schema, diag.Diagnostics) {
	return tfsdk.Schema{
		Attributes: map[string]tfsdk.Attribute{
			"name": {
				Type:          types.StringType,
				Required:      true,
				PlanModifiers: []tfsdk.AttributePlanModifier{resource.RequiresReplace()},
			},
			"id": {
				Type:     types.StringType,
				Computed: true,
			},
			"slug": {
				Type:     types.StringType,
				Computed: true,
			},
		},
	}, nil
}

func (r organizationResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data organizationResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	org, err := r.apiClient.CreateOrganization(ctx, data.Name.Value)
	if err != nil {
		resp.Diagnostics.AddError("failed creating organization", err.Error())
		return
	}
	data.fromApiOrganization(*org)

	resp.Diagnostics.Append(resp.State.Set(ctx, data)...)
}

func (r organizationResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data organizationResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	org, err := r.apiClient.GetOrganizationBySlug(ctx, data.Slug.Value)
	if err != nil {
		resp.Diagnostics.AddError("failed getting organization", err.Error())
		return
	}
	if org == nil {
		resp.State.RemoveResource(ctx)
		return
	}
	data.fromApiOrganization(*org)

	resp.Diagnostics.Append(resp.State.Set(ctx, data)...)
}

func (r organizationResource) Update(context.Context, resource.UpdateRequest, *resource.UpdateResponse) {
}

func (r organizationResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data organizationResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	_, err := r.apiClient.DeleteOrganization(ctx, data.ID.Value)
	if err != nil {
		resp.Diagnostics.AddError("failed deleting organization", err.Error())
		return
	}
}

func newOrganizationResource() resource.Resource {
	return &organizationResource{}
}
