package provider

import (
	"context"
	"errors"
	"fmt"
	"github.com/fly-apps/terraform-provider-fly/graphql"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/vektah/gqlparser/v2/gqlerror"
	"strings"
)

var _ resource.ResourceWithConfigure = &flyCertResource{}
var _ resource.ResourceWithImportState = &flyCertResource{}

type flyCertResource struct {
	flyResource
}

func newFlyCertResource() resource.Resource {
	return &flyCertResource{}
}

type flyCertResourceData struct {
	Id                        types.String `tfsdk:"id"`
	Appid                     types.String `tfsdk:"app"`
	DnsValidationInstructions types.String `tfsdk:"dns_validation_instructions"`
	DnsValidationHostname     types.String `tfsdk:"dns_validation_hostname"`
	DnsValidationTarget       types.String `tfsdk:"dns_validation_target"`
	Hostname                  types.String `tfsdk:"hostname"`
	Check                     types.Bool   `tfsdk:"check"`
}

func (cr flyCertResource) Metadata(_ context.Context, _ resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = "fly_cert"
}

func (flyCertResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"app": schema.StringAttribute{
				MarkdownDescription: APP_DESC,
				Required:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"hostname": schema.StringAttribute{
				Required:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"id": schema.StringAttribute{
				MarkdownDescription: ID_DESC,
				Computed:            true,
			},
			"dns_validation_instructions": schema.StringAttribute{
				Computed: true,
			},
			"dns_validation_target": schema.StringAttribute{
				Computed: true,
			},
			"dns_validation_hostname": schema.StringAttribute{
				Computed: true,
			},
			"check": schema.BoolAttribute{
				Computed: true,
			},
		},
	}
}

func (cr flyCertResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data flyCertResourceData

	diags := req.Plan.Get(ctx, &data)
	resp.Diagnostics.Append(diags...)

	q, err := graphql.AddCertificate(context.Background(), cr.gqlClient, data.Appid.ValueString(), data.Hostname.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Failed to create cert", err.Error())
	}

	data = flyCertResourceData{
		Id:                        types.StringValue(q.AddCertificate.Certificate.Id),
		Appid:                     types.StringValue(data.Appid.ValueString()),
		DnsValidationInstructions: types.StringValue(q.AddCertificate.Certificate.DnsValidationInstructions),
		DnsValidationHostname:     types.StringValue(q.AddCertificate.Certificate.DnsValidationHostname),
		DnsValidationTarget:       types.StringValue(q.AddCertificate.Certificate.DnsValidationTarget),
		Hostname:                  types.StringValue(q.AddCertificate.Certificate.Hostname),
		Check:                     types.BoolValue(q.AddCertificate.Certificate.Check),
	}

	tflog.Info(ctx, fmt.Sprintf("%+v", data))

	diags = resp.State.Set(ctx, &data)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}

func (cr flyCertResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data flyCertResourceData

	diags := req.State.Get(ctx, &data)
	resp.Diagnostics.Append(diags...)

	hostname := data.Hostname.ValueString()
	app := data.Appid.ValueString()

	query, err := graphql.GetCertificate(context.Background(), cr.gqlClient, app, hostname)
	var errList gqlerror.List
	if errors.As(err, &errList) {
		for _, err := range errList {
			if err.Message == "Could not resolve " {
				return
			}
			resp.Diagnostics.AddError(err.Message, err.Path.String())
		}
	} else if err != nil {
		resp.Diagnostics.AddError("Read: query failed", err.Error())
	}

	data = flyCertResourceData{
		Id:                        types.StringValue(query.App.Certificate.Id),
		Appid:                     types.StringValue(data.Appid.ValueString()),
		DnsValidationInstructions: types.StringValue(query.App.Certificate.DnsValidationInstructions),
		DnsValidationHostname:     types.StringValue(query.App.Certificate.DnsValidationHostname),
		DnsValidationTarget:       types.StringValue(query.App.Certificate.DnsValidationTarget),
		Hostname:                  types.StringValue(query.App.Certificate.Hostname),
		Check:                     types.BoolValue(query.App.Certificate.Check),
	}

	diags = resp.State.Set(ctx, &data)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}

func (cr flyCertResource) Update(context.Context, resource.UpdateRequest, *resource.UpdateResponse) {}

func (cr flyCertResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data flyCertResourceData

	diags := req.State.Get(ctx, &data)
	resp.Diagnostics.Append(diags...)

	_, err := graphql.DeleteCertificate(context.Background(), cr.gqlClient, data.Appid.ValueString(), data.Hostname.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Delete cert failed", err.Error())
	}

	resp.State.RemoveResource(ctx)

	if resp.Diagnostics.HasError() {
		return
	}
}

func (cr flyCertResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	idParts := strings.Split(req.ID, ",")

	if len(idParts) != 2 || idParts[0] == "" || idParts[1] == "" {
		resp.Diagnostics.AddError(
			"Unexpected Import Identifier",
			fmt.Sprintf("Expected import identifier with format: app_id,hostname. Got: %q", req.ID),
		)
		return
	}

	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("app"), idParts[0])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("hostname"), idParts[1])...)
}
