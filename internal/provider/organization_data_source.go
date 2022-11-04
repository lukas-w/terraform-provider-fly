package provider

import (
	"context"
	"github.com/fly-apps/terraform-provider-fly/graphql"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
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
	Members []orgMember  `tfsdk:"member"`
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
	specified := 3
	if data.Name.IsUnknown() || data.Name.IsNull() {
		specified--
	}
	if data.Slug.IsUnknown() || data.Slug.IsNull() {
		specified--
	}
	if data.ID.IsUnknown() || data.ID.IsNull() {
		specified--
	}
	if specified != 1 {
		resp.Diagnostics.AddError("Invalid configuration", "Exactly one of name, slug or id must be specified")
	}
}

func newOrganizationDataSource() datasource.DataSource {
	return &organizationDataSource{}
}

func (s organizationDataSource) Metadata(ctx context.Context, request datasource.MetadataRequest, response *datasource.MetadataResponse) {
	response.TypeName = "fly_organization"
}

func (s organizationDataSource) GetSchema(ctx context.Context) (tfsdk.Schema, diag.Diagnostics) {
	return tfsdk.Schema{
		Attributes: map[string]tfsdk.Attribute{
			"name": {
				Type:     types.StringType,
				Optional: true,
				Computed: true,
			},
			"slug": {
				Type:     types.StringType,
				Optional: true,
				Computed: true,
			},
			"id": {
				Type:     types.StringType,
				Optional: true,
				Computed: true,
			},
			"type": {
				Type:                types.StringType,
				Computed:            true,
				Description:         "Organization type, either \"PRIVATE\" or \"SHARED\"",
				MarkdownDescription: "Organization type, either `\"PRIVATE\"` or `\"SHARED\"`",
			},
			"members": {
				Computed: true,
				Attributes: tfsdk.ListNestedAttributes(map[string]tfsdk.Attribute{
					"id": {
						Type:     types.StringType,
						Computed: true,
					},
					"name": {
						Type:     types.StringType,
						Computed: true,
					},
					"email": {
						Type:     types.StringType,
						Computed: true,
					},
					"joined_at": {
						Type:     types.StringType,
						Computed: true,
					},
					"role": {
						Type:                types.StringType,
						Computed:            true,
						Description:         "Member role, either \"ADMIN\" or \"MEMBER\"",
						MarkdownDescription: "Member role, either `\"ADMIN\"` or `\"MEMBER\"`",
					},
				}),
			},
		},
	}, nil
}

func (s organizationDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data organizationDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)

	variables := make(map[string]interface{})
	if !data.ID.IsNull() && !data.ID.IsUnknown() {
		variables["id"] = data.ID.Value
	} else if !data.Slug.IsNull() && !data.Slug.IsUnknown() {
		variables["slug"] = data.Slug.Value
	} else if !data.Name.IsNull() && !data.Name.IsUnknown() {
		variables["name"] = data.Name.Value
	}

	r, err := graphql.Organization(ctx, s.gqlClient, data.ID.Value, data.Slug.Value, data.Name.Value)
	if err != nil {
		resp.Diagnostics.AddError("Error reading organization", err.Error())
		return
	}
	if r == nil {
		resp.State.RemoveResource(ctx)
		return
	}

	data.ID = types.String{Value: r.Organization.Id}
	data.Slug = types.String{Value: r.Organization.Slug}
	data.Name = types.String{Value: r.Organization.Name}
	data.Type = types.String{Value: string(r.Organization.Type)}

	data.Members = make([]orgMember, len(r.Organization.Members.Edges))
	for i, m := range r.Organization.Members.Edges {
		data.Members[i] = orgMember{
			Id:       types.String{Value: m.Node.Id},
			Name:     types.String{Value: m.Node.Name},
			Email:    types.String{Value: m.Node.Email},
			JoinedAt: types.String{Value: m.JoinedAt.Format(time.RFC3339)},
			Role:     types.String{Value: string(m.Role)},
		}
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
