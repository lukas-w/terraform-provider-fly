package provider

import (
	"context"
	"github.com/fly-apps/terraform-provider-fly/graphql"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"time"
)

type orgMember struct {
	Id       types.String `tfsdk:"id"`
	Name     types.String `tfsdk:"name"`
	Email    types.String `tfsdk:"email"`
	JoinedAt types.String `tfsdk:"joined_at"`
	Role     types.String `tfsdk:"role"`
}

type organizationDataSourceModel struct {
	Name    types.String `tfsdk:"name"`
	Slug    types.String `tfsdk:"slug"`
	ID      types.String `tfsdk:"id"`
	Type    types.String `tfsdk:"type"`
	Members []orgMember  `tfsdk:"members"`
}

type organizationDataSource struct {
	flyDataSource
}

func (s organizationDataSource) ValidateConfig(ctx context.Context, req datasource.ValidateConfigRequest, resp *datasource.ValidateConfigResponse) {
	var data organizationDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	unspecified := 0
	if data.Name.IsUnknown() || data.Name.IsNull() {
		resp.Diagnostics.AddWarning("Unspecified attribute", "name")
		unspecified++
	}
	if data.Slug.IsUnknown() || data.Slug.IsNull() {
		resp.Diagnostics.AddWarning("Unspecified attribute", "slug")
		unspecified++
	}
	if data.ID.IsUnknown() || data.ID.IsNull() {
		resp.Diagnostics.AddWarning("Unspecified attribute", "id")
		unspecified++
	}
	if unspecified != 2 {
		resp.Diagnostics.AddError("Invalid configuration", "Exactly one of name, slug or id must be specified")
	}
}

func newOrganizationDataSource() datasource.DataSource {
	return &organizationDataSource{}
}

func (s organizationDataSource) Metadata(_ context.Context, _ datasource.MetadataRequest, response *datasource.MetadataResponse) {
	response.TypeName = "fly_organization"
}

func (s organizationDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"name": schema.StringAttribute{
				Optional: true,
				Computed: true,
			},
			"slug": schema.StringAttribute{
				Optional: true,
				Computed: true,
			},
			"id": schema.StringAttribute{
				Optional: true,
				Computed: true,
			},
			"type": schema.StringAttribute{
				Computed:            true,
				Description:         "Organization type, either \"PRIVATE\" or \"SHARED\"",
				MarkdownDescription: "Organization type, either `\"PRIVATE\"` or `\"SHARED\"`",
			},
			"members": schema.ListNestedAttribute{
				Computed: true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"id": schema.StringAttribute{
							Computed: true,
						},
						"name": schema.StringAttribute{
							Computed: true,
						},
						"email": schema.StringAttribute{
							Computed: true,
						},
						"joined_at": schema.StringAttribute{
							Computed: true,
						},
						"role": schema.StringAttribute{
							Computed:            true,
							Description:         "Member role, either \"ADMIN\" or \"MEMBER\"",
							MarkdownDescription: "Member role, either `\"ADMIN\"` or `\"MEMBER\"`",
						},
					},
				},
			},
		},
	}
}

func (s organizationDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data organizationDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)

	variables := make(map[string]interface{})
	if !data.ID.IsNull() && !data.ID.IsUnknown() {
		variables["id"] = data.ID.ValueString()
	} else if !data.Slug.IsNull() && !data.Slug.IsUnknown() {
		variables["slug"] = data.Slug.ValueString()
	} else if !data.Name.IsNull() && !data.Name.IsUnknown() {
		variables["name"] = data.Name.ValueString()
	}

	r, err := graphql.Organization(ctx, s.gqlClient, data.ID.ValueString(), data.Slug.ValueString(), data.Name.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error reading organization", err.Error())
		return
	}
	if r == nil {
		resp.State.RemoveResource(ctx)
		return
	}

	data.ID = types.StringValue(r.Organization.Id)
	data.Slug = types.StringValue(r.Organization.Slug)
	data.Name = types.StringValue(r.Organization.Name)
	data.Type = types.StringValue(string(r.Organization.Type))

	data.Members = make([]orgMember, len(r.Organization.Members.Edges))
	for i, m := range r.Organization.Members.Edges {
		data.Members[i] = orgMember{
			Id:       types.StringValue(m.Node.Id),
			Name:     types.StringValue(m.Node.Name),
			Email:    types.StringValue(m.Node.Email),
			JoinedAt: types.StringValue(m.JoinedAt.Format(time.RFC3339)),
			Role:     types.StringValue(string(m.Role)),
		}
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
