package provider

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/fly-apps/terraform-provider-fly/graphql"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/vektah/gqlparser/v2/gqlerror"
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

func (r *flyIpResourceData) fromApi(f graphql.IPAddressFragment) {
	r.Id = types.StringValue(f.Id)
	r.Region = types.StringValue(f.Region)
	r.Address = types.StringValue(f.Address)
	r.Type = types.StringValue(string(f.Type))
	if f.Type == "private_v6" {
		r.Type = types.StringValue("v6")
		r.Private = types.BoolValue(true)
	} else if f.Type == "v6" {
		r.Type = types.StringValue("v6")
		r.Private = types.BoolValue(isPrivate(f.Address))
	} else if f.Type == "v4" {
		r.Type = types.StringValue("v4")
	} else if f.Type == "shared_v4" {
		r.Type = types.StringValue("v4")
	}
}

func (ir flyIpResource) Metadata(_ context.Context, _ resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = "fly_ip"
}

func (flyIpResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Fly ip resource",
		Attributes: map[string]schema.Attribute{
			"address": schema.StringAttribute{
				Computed: true,
			},
			"app": schema.StringAttribute{
				MarkdownDescription: APP_DESC,
				Required:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"id": schema.StringAttribute{
				MarkdownDescription: ID_DESC,
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"type": schema.StringAttribute{
				MarkdownDescription: "v4 or v6",
				Required:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"private": schema.BoolAttribute{
				Optional: true,
				Computed: true,
				Default:  booldefault.StaticBool(false),
			},
			"region": schema.StringAttribute{
				MarkdownDescription: REGION_DESC,
				Computed:            true,
				Default:             stringdefault.StaticString("global"),
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
		},
	}
}

func (ir flyIpResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data flyIpResourceData

	diags := req.Plan.Get(ctx, &data)
	resp.Diagnostics.Append(diags...)

	tflog.Info(ctx, fmt.Sprintf("%+v", data))

	var ipType graphql.IPAddressType
	if data.Type.ValueString() == "v4" {
		ipType = graphql.IPAddressTypeV4
	} else if data.Private.ValueBool() {
		ipType = graphql.IPAddressTypePrivateV6
	} else {
		ipType = graphql.IPAddressTypeV6
	}

	q, err := graphql.AllocateIpAddress(context.Background(), ir.gqlClient, data.Appid.ValueString(), data.Region.ValueString(), ipType)
	tflog.Info(ctx, fmt.Sprintf("query res in create ip: %+v", q))
	if err != nil {
		resp.Diagnostics.AddError("Failed to create ip addr", err.Error())
	}

	data.fromApi(q.AllocateIpAddress.IpAddress)

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

	query, err := graphql.IpAddressQuery(context.Background(), ir.gqlClient, data.Id.ValueString())
	tflog.Info(ctx, fmt.Sprintf("Query res: for %s %+v", data.Id.ValueString(), query))
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

	data.fromApi(query.IpAddress)

	diags = resp.State.Set(ctx, &data)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}

func (ir flyIpResource) Update(_ context.Context, _ resource.UpdateRequest, resp *resource.UpdateResponse) {
	resp.Diagnostics.AddError("The fly api does not allow updating ips once created", "Try deleting and then recreating the ip with new options")
	return
}

func (ir flyIpResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data flyIpResourceData

	diags := req.State.Get(ctx, &data)
	resp.Diagnostics.Append(diags...)

	if data.Id.ValueString() != "" {
		_, err := graphql.ReleaseIpAddress(context.Background(), ir.gqlClient, data.Id.ValueString())
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
	idParts := strings.Split(req.ID, ",")

	if len(idParts) != 2 || idParts[0] == "" || idParts[1] == "" {
		resp.Diagnostics.AddError(
			"Unexpected Import Identifier",
			fmt.Sprintf("Expected import identifier with format: app_id,ip_address. Got: %q", req.ID),
		)
		return
	}

	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("app"), idParts[0])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("address"), idParts[1])...)
}

func isPrivate(address string) bool {
	return strings.HasPrefix(address, "fdaa")
}
