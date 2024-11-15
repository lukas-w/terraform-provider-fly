package provider

import (
	"context"
	"errors"
	"fmt"
	"github.com/fly-apps/terraform-provider-fly/graphql"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/vektah/gqlparser/v2/gqlerror"
)

var _ datasource.DataSourceWithConfigure = &ipDataSource{}

// Matches getSchema
type ipDataSourceOutput struct {
	Id      types.String `tfsdk:"id"`
	Appid   types.String `tfsdk:"app"`
	Region  types.String `tfsdk:"region"`
	Address types.String `tfsdk:"address"`
	Type    types.String `tfsdk:"type"`
}

func (i ipDataSource) Metadata(_ context.Context, _ datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = "fly_ip"
}

func (i ipDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Fly ip data source",
		Attributes: map[string]schema.Attribute{
			"address": schema.StringAttribute{
				MarkdownDescription: "ID of volume",
				Computed:            true,
			},
			"app": schema.StringAttribute{
				MarkdownDescription: "Name of app to attach",
				Required:            true,
			},
			"id": schema.StringAttribute{
				MarkdownDescription: "ID of address",
				Required:            true,
			},
			"type": schema.StringAttribute{
				MarkdownDescription: "v4 or v6",
				Computed:            true,
			},
			"region": schema.StringAttribute{
				MarkdownDescription: "region",
				Computed:            true,
			},
		},
	}
}

func newIpDataSource() datasource.DataSource {
	return &ipDataSource{}
}

func (i ipDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data ipDataSourceOutput

	diags := req.Config.Get(ctx, &data)
	resp.Diagnostics.Append(diags...)

	if resp.Diagnostics.HasError() {
		return
	}

	query, err := graphql.IpAddressQuery(context.Background(), i.gqlClient, data.Id.ValueString())
	tflog.Info(ctx, fmt.Sprintf("Query res: for %s %+v", data.Id.ValueString(), query))
	var errList gqlerror.List
	if errors.As(err, &errList) {
		for _, err := range errList {
			tflog.Info(ctx, "IN HERE")
			if err.Message == "Could not resolve " {
				return
			}
			resp.Diagnostics.AddError(err.Message, err.Path.String())
		}
	} else if err != nil {
		resp.Diagnostics.AddError("Read: query failed", err.Error())
	}

	data = ipDataSourceOutput{
		Id:      types.StringValue(query.IpAddress.Id),
		Appid:   types.StringValue(data.Appid.ValueString()),
		Region:  types.StringValue(query.IpAddress.Region),
		Type:    types.StringValue(string(query.IpAddress.Type)),
		Address: types.StringValue(query.IpAddress.Address),
	}

	diags = resp.State.Set(ctx, &data)
	resp.Diagnostics.Append(diags...)
}
