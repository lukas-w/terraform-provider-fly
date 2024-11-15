package provider

import (
	"context"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

type latestImageTagDataSourceModel struct {
	Repository string       `tfsdk:"repository"`
	SnapshotID *string      `tfsdk:"snapshot_id"`
	ImageRef   types.String `tfsdk:"image_ref"`
}

type latestImageTagDataSource struct {
	flyDataSource
}

func newLatestImageTagDataSource() datasource.DataSource {
	return &latestImageTagDataSource{}
}

func (l latestImageTagDataSource) Metadata(_ context.Context, _ datasource.MetadataRequest, response *datasource.MetadataResponse) {
	response.TypeName = "fly_latest_image_tag"
}

func (l latestImageTagDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"repository": schema.StringAttribute{
				Required: true,
			},
			"snapshot_id": schema.StringAttribute{
				Optional: true,
			},
			"image_ref": schema.StringAttribute{
				Computed: true,
			},
		},
	}
}

func (l latestImageTagDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data latestImageTagDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	t, err := l.apiClient.GetLatestImageTag(ctx, data.Repository, data.SnapshotID)
	if err != nil {
		resp.Diagnostics.AddError("error in GetLatestImageTag API call", err.Error())
		return
	}
	data.ImageRef = types.StringValue(t)

	resp.Diagnostics.Append(resp.State.Set(ctx, data)...)
}
