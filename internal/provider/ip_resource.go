package provider

import (
	"context"
	"errors"
	"fmt"
	"github.com/fly-apps/terraform-provider-fly/graphql"
	"github.com/fly-apps/terraform-provider-fly/internal/provider/modifiers"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/vektah/gqlparser/v2/gqlerror"
	"strings"
)

// Note that there are inconsistencies in GraphQL API's behaviour regarding the ip address type:
// The AllocateIpAddress expects v4, v6 or private_v6, but all data returned contains either v4 or v6.
// We can only detect a private v6 IP by checking for a `fdaa` prefix like flyctl does.

var _ resource.ResourceWithConfigure = &flyIpResource{}
var _ resource.ResourceWithImportState = &flyIpResource{}

type flyIpResource struct {
	flyResource
}

func newFlyIpResource() resource.Resource {
	return &flyIpResource{}
}

type flyIpResourceData struct {
	Id      types.String `tfsdk:"id"`
	Appid   types.String `tfsdk:"app"`
	Region  types.String `tfsdk:"region"`
	Address types.String `tfsdk:"address"`
	Type    types.String `tfsdk:"type"`
	Private types.Bool   `tfsdk:"private"`
}

func (ir flyIpResource) Metadata(_ context.Context, _ resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = "fly_ip"
}

func (flyIpResource) GetSchema(context.Context) (tfsdk.Schema, diag.Diagnostics) {
	return tfsdk.Schema{
		MarkdownDescription: "Fly ip resource",
		Attributes: map[string]tfsdk.Attribute{
			"address": {
				MarkdownDescription: "IP address",
				Type:                types.StringType,
				Computed:            true,
			},
			"app": {
				MarkdownDescription: "Name of app to attach to",
				Required:            true,
				Type:                types.StringType,
				PlanModifiers:       []tfsdk.AttributePlanModifier{resource.RequiresReplace()},
			},
			"id": {
				MarkdownDescription: "ID of address",
				Computed:            true,
				Type:                types.StringType,
				PlanModifiers:       []tfsdk.AttributePlanModifier{resource.RequiresReplace()},
			},
			"type": {
				MarkdownDescription: "v4 or v6",
				Type:                types.StringType,
				Required:            true,
				PlanModifiers:       []tfsdk.AttributePlanModifier{resource.RequiresReplace()},
			},
			"private": {
				Type:          types.BoolType,
				Optional:      true,
				Computed:      true,
				PlanModifiers: []tfsdk.AttributePlanModifier{modifiers.Default(types.Bool{Value: false})},
			},
			"region": {
				MarkdownDescription: "region",
				Type:                types.StringType,
				Computed:            true,
				PlanModifiers: tfsdk.AttributePlanModifiers{
					resource.RequiresReplace(),
					modifiers.Default(types.String{Value: "global"}),
				},
			},
		},
	}, nil
}

func (ir flyIpResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data flyIpResourceData

	diags := req.Plan.Get(ctx, &data)
	resp.Diagnostics.Append(diags...)

	tflog.Info(ctx, fmt.Sprintf("%+v", data))

	var ipType graphql.IPAddressType
	if data.Type.Value == "v4" {
		ipType = graphql.IPAddressTypeV4
	} else if data.Private.Value {
		ipType = graphql.IPAddressTypePrivateV6
	} else {
		ipType = graphql.IPAddressTypeV6
	}

	q, err := graphql.AllocateIpAddress(context.Background(), ir.gqlClient, data.Appid.Value, data.Region.Value, ipType)
	tflog.Info(ctx, fmt.Sprintf("query res in create ip: %+v", q))
	if err != nil {
		resp.Diagnostics.AddError("Failed to create ip addr", err.Error())
	}

	data = flyIpResourceData{
		Id:      types.String{Value: q.AllocateIpAddress.IpAddress.Id},
		Appid:   types.String{Value: data.Appid.Value},
		Region:  types.String{Value: q.AllocateIpAddress.IpAddress.Region},
		Type:    types.String{Value: string(q.AllocateIpAddress.IpAddress.Type)},
		Address: types.String{Value: q.AllocateIpAddress.IpAddress.Address},
		Private: types.Bool{Value: isPrivate(q.AllocateIpAddress.IpAddress.Address)},
	}

	tflog.Info(ctx, fmt.Sprintf("%+v", data))

	diags = resp.State.Set(ctx, &data)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}

func (ir flyIpResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data flyIpResourceData

	diags := req.State.Get(ctx, &data)
	resp.Diagnostics.Append(diags...)

	query, err := graphql.IpAddressQuery(context.Background(), ir.gqlClient, data.Id.Value)
	tflog.Info(ctx, fmt.Sprintf("Query res: for %s %+v", data.Id.Value, query))
	var errList gqlerror.List
	if errors.As(err, &errList) {
		for _, err := range errList {
			if strings.Contains(strings.ToLower(err.Message), "could not find") {
				resp.State.RemoveResource(ctx)
				return
			}
			resp.Diagnostics.AddError(err.Message, err.Path.String())
		}
	} else if err != nil {
		resp.Diagnostics.AddError("Read: query failed", err.Error())
	}

	data = flyIpResourceData{
		Id:      types.String{Value: query.IpAddress.Id},
		Appid:   types.String{Value: data.Appid.Value},
		Region:  types.String{Value: query.IpAddress.Region},
		Type:    types.String{Value: string(query.IpAddress.Type)},
		Address: types.String{Value: query.IpAddress.Address},
		Private: types.Bool{Value: isPrivate(query.IpAddress.Address)},
	}

	diags = resp.State.Set(ctx, &data)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}

func (ir flyIpResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	resp.Diagnostics.AddError("The fly api does not allow updating ips once created", "Try deleting and then recreating the ip with new options")
	return
}

func (ir flyIpResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data flyIpResourceData

	diags := req.State.Get(ctx, &data)
	resp.Diagnostics.Append(diags...)

	if !data.Id.Unknown && !data.Id.Null && data.Id.Value != "" {
		_, err := graphql.ReleaseIpAddress(context.Background(), ir.gqlClient, data.Id.Value)
		if err != nil {
			resp.Diagnostics.AddError("Release ip failed", err.Error())
		}
	}

	resp.State.RemoveResource(ctx)

	if resp.Diagnostics.HasError() {
		return
	}
}

func (ir flyIpResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func isPrivate(address string) bool {
	return strings.HasPrefix(address, "fdaa")
}
