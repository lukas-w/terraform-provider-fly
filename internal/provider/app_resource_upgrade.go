package provider

import (
	"context"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// V0->V1: Add enable_postgres_consul
// V1->V2: Add preferred_region, rename enable_postgres_consul to enable_consul
// V2->V3: Remove machines

type flyAppResourceDataV1 struct {
	Name                 types.String `tfsdk:"name"`
	Org                  types.String `tfsdk:"org"`
	OrgId                types.String `tfsdk:"orgid"`
	AppUrl               types.String `tfsdk:"appurl"`
	Id                   types.String `tfsdk:"id"`
	Machines             types.Bool   `tfsdk:"machines"`
	Secrets              secretMap    `tfsdk:"secrets"`
	RoleId               types.String `tfsdk:"role_id"`
	EnablePostgresConsul types.Bool   `tfsdk:"enable_postgres_consul"`
	ConsulUrl            types.String `tfsdk:"consul_url"`
}

type flyAppResourceDataV2 struct {
	Name            types.String `tfsdk:"name"`
	Org             types.String `tfsdk:"org"`
	OrgId           types.String `tfsdk:"orgid"`
	PreferredRegion types.String `tfsdk:"preferred_region"`
	AppUrl          types.String `tfsdk:"appurl"`
	Id              types.String `tfsdk:"id"`
	Machines        types.Bool   `tfsdk:"machines"`
	Secrets         secretMap    `tfsdk:"secrets"`
	RoleId          types.String `tfsdk:"role_id"`
	EnableConsul    types.Bool   `tfsdk:"enable_consul"`
	ConsulUrl       types.String `tfsdk:"consul_url"`
}

func (r flyAppResource) UpgradeState(ctx context.Context) map[int64]resource.StateUpgrader {
	return map[int64]resource.StateUpgrader{
		0: {
			StateUpgrader: func(ctx context.Context, req resource.UpgradeStateRequest, resp *resource.UpgradeStateResponse) {
				var data flyAppResourceData
				resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
				data.EnableConsul = types.BoolNull()
				resp.Diagnostics.Append(resp.State.Set(ctx, data)...)
			},
		},
		1: {
			PriorSchema: &tfsdk.Schema{
				Attributes: map[string]tfsdk.Attribute{
					"name": {
						Required: true,
						Type:     types.StringType,
					},
					"org": {
						Computed: true,
						Optional: true,
						Type:     types.StringType,
					},
					"machines": {
						Optional: true,
						Computed: true,
						Type:     types.BoolType,
					},
					"secrets": {
						Optional: true,
						Attributes: tfsdk.MapNestedAttributes(map[string]tfsdk.Attribute{
							"value": {
								Type:      types.StringType,
								Sensitive: true,
								Required:  true,
							},
							"digest": {
								Type:     types.StringType,
								Computed: true,
							},
							"created_at": {
								Type:     types.StringType,
								Computed: true,
							},
						}),
					},
					"orgid": {
						Computed: true,
						Type:     types.StringType,
					},
					"id": {
						Computed: true,
						Type:     types.StringType,
					},
					"appurl": {
						Computed: true,
						Type:     types.StringType,
					},
					"role_id": {
						Type:     types.StringType,
						Optional: true,
					},
					"enable_postgres_consul": {
						Type:     types.BoolType,
						Optional: true,
						Computed: true,
					},
					"consul_url": {
						Type:      types.StringType,
						Computed:  true,
						Sensitive: true,
					},
				},
			},
			StateUpgrader: func(ctx context.Context, req resource.UpgradeStateRequest, resp *resource.UpgradeStateResponse) {
				var d flyAppResourceDataV1
				resp.Diagnostics.Append(req.State.Get(ctx, &d)...)
				resp.Diagnostics.Append(resp.State.Set(ctx, flyAppResourceData{
					Name:         d.Name,
					Org:          d.Org,
					OrgId:        d.OrgId,
					AppUrl:       d.AppUrl,
					Id:           d.Id,
					Secrets:      d.Secrets,
					RoleId:       d.RoleId,
					EnableConsul: d.EnablePostgresConsul,
					ConsulUrl:    d.ConsulUrl,
				})...)
			},
		},
		2: {
			PriorSchema: &tfsdk.Schema{
				Attributes: map[string]tfsdk.Attribute{
					"name": {
						Required: true,
						Type:     types.StringType,
					},
					"org": {
						Computed: true,
						Optional: true,
						Type:     types.StringType,
					},
					"preferred_region": {
						Optional: true,
						Computed: true,
						Type:     types.StringType,
					},
					"machines": {
						Optional: true,
						Computed: true,
						Type:     types.BoolType,
					},
					"secrets": {
						Optional: true,
						Attributes: tfsdk.MapNestedAttributes(map[string]tfsdk.Attribute{
							"value": {
								Type:      types.StringType,
								Sensitive: true,
								Required:  true,
							},
							"digest": {
								Type:     types.StringType,
								Computed: true,
							},
							"created_at": {
								Type:     types.StringType,
								Computed: true,
							},
						}),
					},
					"orgid": {
						Computed: true,
						Type:     types.StringType,
					},
					"id": {
						Computed: true,
						Type:     types.StringType,
					},
					"appurl": {
						Computed: true,
						Type:     types.StringType,
					},
					"role_id": {
						Type:     types.StringType,
						Optional: true,
					},
					"enable_postgres_consul": {
						Type:     types.BoolType,
						Optional: true,
						Computed: true,
					},
					"consul_url": {
						Type:      types.StringType,
						Computed:  true,
						Sensitive: true,
					},
				},
			},
			StateUpgrader: func(ctx context.Context, req resource.UpgradeStateRequest, resp *resource.UpgradeStateResponse) {
				var d flyAppResourceDataV2
				resp.Diagnostics.Append(req.State.Get(ctx, &d)...)
				resp.Diagnostics.Append(resp.State.Set(ctx, flyAppResourceData{
					Name:            d.Name,
					Org:             d.Org,
					OrgId:           d.OrgId,
					PreferredRegion: d.PreferredRegion,
					AppUrl:          d.AppUrl,
					Id:              d.Id,
					Secrets:         d.Secrets,
					RoleId:          d.RoleId,
					EnableConsul:    d.EnableConsul,
					ConsulUrl:       d.ConsulUrl,
				})...)
			},
		},
	}
}
