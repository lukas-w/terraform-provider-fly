package provider

import (
	"context"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
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

func (l latestImageTagDataSource) GetSchema(context.Context) (tfsdk.Schema, diag.Diagnostics) {
	return tfsdk.Schema{
		Attributes: map[string]tfsdk.Attribute{
			"repository": {
				Required: true,
				Type:     types.StringType,
			},
			"snapshot_id": {
				Optional: true,
				Type:     types.StringType,
			},
			"image_ref": {
				Computed: true,
				Type:     types.StringType,
			},
		},
	}, nil
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
	data.ImageRef = types.String{Value: t}

	resp.Diagnostics.Append(resp.State.Set(ctx, data)...)
}
