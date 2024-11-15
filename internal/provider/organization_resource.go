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

type organizationResourceModel struct {
	Name types.String `tfsdk:"name"`
	ID   types.String `tfsdk:"id"`
	Slug types.String `tfsdk:"slug"`
}
type organizationResource struct {
	flyResource
}

func (m *organizationResourceModel) fromApiOrganization(org fly.Organization) {
	m.Name = types.StringValue(org.Name)
	m.ID = types.StringValue(org.ID)
	m.Slug = types.StringValue(org.Slug)
}

func (r organizationResource) Metadata(_ context.Context, _ resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = "fly_organization"
}

func (r organizationResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"name": schema.StringAttribute{
				Required:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"id": schema.StringAttribute{
				Computed: true,
			},
			"slug": schema.StringAttribute{
				Computed: true,
			},
		},
	}
}

func (r organizationResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data organizationResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	org, err := r.apiClient.CreateOrganization(ctx, data.Name.ValueString())
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

	org, err := r.apiClient.GetOrganizationBySlug(ctx, data.Slug.ValueString())
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

	_, err := r.apiClient.DeleteOrganization(ctx, data.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("failed deleting organization", err.Error())
		return
	}
}

func newOrganizationResource() resource.Resource {
	return &organizationResource{}
}
