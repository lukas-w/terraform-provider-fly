package provider

import (
	"context"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
)

type postgresConsulResource struct {
	flyResource
}

func (p postgresConsulResource) Metadata(_ context.Context, _ resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = "fly_postgres_consul"
}

func (p postgresConsulResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"app": schema.StringAttribute{
				Required: true,
			},
			"url": schema.StringAttribute{
				Computed:  true,
				Sensitive: true,
			},
		},
	}
}

func (p postgresConsulResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var app string
	resp.Diagnostics.Append(req.Plan.GetAttribute(ctx, path.Root("app"), &app)...)
	if resp.Diagnostics.HasError() {
		return
	}
	r, err := p.apiClient.EnablePostgresConsul(ctx, app)
	if err != nil {
		resp.Diagnostics.AddError("error in API call EnablePostgresConsul", err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("app"), app)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("url"), r.ConsulURL)...)
}

func (p postgresConsulResource) Read(context.Context, resource.ReadRequest, *resource.ReadResponse) {
}

func (p postgresConsulResource) Update(context.Context, resource.UpdateRequest, *resource.UpdateResponse) {
}

func (p postgresConsulResource) Delete(context.Context, resource.DeleteRequest, *resource.DeleteResponse) {
}

func newPostgresConsulResource() resource.Resource {
	return &postgresConsulResource{}
}
