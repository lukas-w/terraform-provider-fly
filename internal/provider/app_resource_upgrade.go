package provider

import (
	"context"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// V0->V1: Add enable_postgres_consul
// V1->V2: Add preferred_region, rename enable_postgres_consul to enable_consul
// V2->V3: Remove machines & preferred_region

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

func (r flyAppResource) UpgradeState(context.Context) map[int64]resource.StateUpgrader {
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
			PriorSchema: &schema.Schema{
				Attributes: map[string]schema.Attribute{
					"name": schema.StringAttribute{
						Required: true,
					},
					"org": schema.StringAttribute{
						Computed: true,
						Optional: true,
					},
					"machines": schema.BoolAttribute{
						Optional: true,
						Computed: true,
					},
					"secrets": schema.MapNestedAttribute{
						Optional: true,
						NestedObject: schema.NestedAttributeObject{
							Attributes: map[string]schema.Attribute{
								"value": schema.StringAttribute{
									Sensitive: true,
									Required:  true,
								},
								"digest": schema.StringAttribute{
									Computed: true,
								},
								"created_at": schema.StringAttribute{
									Computed: true,
								},
							},
						},
					},
					"orgid": schema.StringAttribute{
						Computed: true,
					},
					"id": schema.StringAttribute{
						Computed: true,
					},
					"appurl": schema.StringAttribute{
						Computed: true,
					},
					"role_id": schema.StringAttribute{
						Optional: true,
					},
					"enable_postgres_consul": schema.BoolAttribute{
						Optional: true,
						Computed: true,
					},
					"consul_url": schema.StringAttribute{
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
			PriorSchema: &schema.Schema{
				Attributes: map[string]schema.Attribute{
					"name": schema.StringAttribute{
						Required: true,
					},
					"org": schema.StringAttribute{
						Computed: true,
						Optional: true,
					},
					"preferred_region": schema.StringAttribute{
						Optional: true,
						Computed: true,
					},
					"machines": schema.BoolAttribute{
						Optional: true,
						Computed: true,
					},
					"secrets": schema.MapNestedAttribute{
						Optional: true,
						NestedObject: schema.NestedAttributeObject{
							Attributes: map[string]schema.Attribute{
								"value": schema.StringAttribute{
									Sensitive: true,
									Required:  true,
								},
								"digest": schema.StringAttribute{
									Computed: true,
								},
								"created_at": schema.StringAttribute{
									Computed: true,
								},
							},
						},
					},
					"orgid": schema.StringAttribute{
						Computed: true,
					},
					"id": schema.StringAttribute{
						Computed: true,
					},
					"appurl": schema.StringAttribute{
						Computed: true,
					},
					"role_id": schema.StringAttribute{
						Optional: true,
					},
					"enable_consul": schema.BoolAttribute{
						Optional: true,
						Computed: true,
					},
					"consul_url": schema.StringAttribute{
						Computed:  true,
						Sensitive: true,
					},
				},
			},
			StateUpgrader: func(ctx context.Context, req resource.UpgradeStateRequest, resp *resource.UpgradeStateResponse) {
				var d flyAppResourceDataV2
				resp.Diagnostics.Append(req.State.Get(ctx, &d)...)
				resp.Diagnostics.Append(resp.State.Set(ctx, flyAppResourceData{
					Name:         d.Name,
					Org:          d.Org,
					OrgId:        d.OrgId,
					AppUrl:       d.AppUrl,
					Id:           d.Id,
					Secrets:      d.Secrets,
					RoleId:       d.RoleId,
					EnableConsul: d.EnableConsul,
					ConsulUrl:    d.ConsulUrl,
				})...)
			},
		},
	}
}
