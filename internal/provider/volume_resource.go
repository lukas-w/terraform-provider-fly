package provider

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/superfly/fly-go"
)

var _ resource.ResourceWithConfigure = &flyVolumeResource{}
var _ resource.ResourceWithImportState = &flyVolumeResource{}

type flyVolumeResource struct {
	flyResource
}

func newFlyVolumeResource() resource.Resource {
	return &flyVolumeResource{}
}

type flyVolumeResourceData struct {
	Id        types.String `tfsdk:"id"`
	Name      types.String `tfsdk:"name"`
	Size      types.Int64  `tfsdk:"size"`
	Appid     types.String `tfsdk:"app"`
	Region    types.String `tfsdk:"region"`
	Encrypted types.Bool   `tfsdk:"encrypted"`
}

func (d *flyVolumeResourceData) fromApiVolume(v fly.Volume) {
	d.Id = types.StringValue(v.ID)
	d.Name = types.StringValue(v.Name)
	d.Size = types.Int64Value(int64(v.SizeGb))
	d.Region = types.StringValue(v.Region)
	d.Encrypted = types.BoolValue(v.Encrypted)
}

func (vr flyVolumeResource) Metadata(_ context.Context, _ resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = "fly_volume"
}

func (vr flyVolumeResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Fly volume resource",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "ID of volume",
				Computed:            true,
			},
			"app": schema.StringAttribute{
				MarkdownDescription: "Name of app to attach",
				Required:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"size": schema.Int64Attribute{
				MarkdownDescription: "Size of volume in gb",
				Required:            true,
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.RequiresReplaceIf(func(ctx context.Context, req planmodifier.Int64Request, resp *int64planmodifier.RequiresReplaceIfFuncResponse) {
						var size, newSize int64
						resp.Diagnostics.Append(tfsdk.ValueAs(ctx, req.StateValue, &size)...)
						resp.Diagnostics.Append(tfsdk.ValueAs(ctx, req.ConfigValue, &newSize)...)
						resp.RequiresReplace = newSize < size
					}, "volumes cannot be made smaller", "volumes cannot be made smaller"),
				},
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "name",
				Required:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"region": schema.StringAttribute{
				MarkdownDescription: "region",
				Required:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"encrypted": schema.BoolAttribute{
				MarkdownDescription: "Encrypt the volume contents. Defaults to `true`",
				Optional:            true,
				Computed:            true,
			},
		},
	}
}

func (vr flyVolumeResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data flyVolumeResourceData

	diags := req.Plan.Get(ctx, &data)
	resp.Diagnostics.Append(diags...)

	client := vr.getFlapsClient(ctx, data.Appid.ValueString(), &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	sizeGb := int(data.Size.ValueInt64())
	encrypted := data.Encrypted.IsNull() || data.Encrypted.IsUnknown() || data.Encrypted.ValueBool()
	v, err := client.CreateVolume(ctx, fly.CreateVolumeRequest{
		Name:      data.Name.ValueString(),
		Region:    data.Region.ValueString(),
		SizeGb:    &sizeGb,
		Encrypted: &encrypted,
	})
	if err != nil {
		resp.Diagnostics.AddError("Failed to create volume", err.Error())
		return
	}

	data.fromApiVolume(*v)

	tflog.Info(ctx, fmt.Sprintf("%+v", data))

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (vr flyVolumeResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data flyVolumeResourceData

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	client := vr.getFlapsClient(ctx, data.Appid.ValueString(), &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	v, err := client.GetVolume(ctx, data.Id.ValueString())
	if err != nil && strings.ToLower(err.Error()) == "could not find volume" {
		resp.State.RemoveResource(ctx)
		return
	} else if err != nil {
		resp.Diagnostics.AddError("Read: query failed", err.Error())
	}
	if v == nil {
		resp.State.RemoveResource(ctx)
		return
	}

	data.fromApiVolume(*v)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (vr flyVolumeResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var state, plan flyVolumeResourceData
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	client := vr.getFlapsClient(ctx, state.Appid.ValueString(), &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	if plan.Size.ValueInt64() > state.Size.ValueInt64() {
		v, needsRestart, err := client.ExtendVolume(ctx, state.Id.ValueString(), int(plan.Size.ValueInt64()))
		if err != nil {
			resp.Diagnostics.AddError("error extending volume", err.Error())
			return
		}
		if needsRestart {
			resp.Diagnostics.AddWarning(
				"app needs restart",
				fmt.Sprintf("app %s needs to be restarted after extending volume %s with ID %s", state.Appid.ValueString(), state.Name.ValueString(), state.Id.ValueString()),
			)
		}
		state.fromApiVolume(*v)
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (vr flyVolumeResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data flyVolumeResourceData
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	client := vr.getFlapsClient(ctx, data.Appid.ValueString(), &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	_, err := client.DeleteVolume(ctx, data.Id.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Delete volume failed", err.Error())
		return
	}
}

func (vr flyVolumeResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	idParts := strings.Split(req.ID, ",")

	if len(idParts) != 2 || idParts[0] == "" || idParts[1] == "" {
		resp.Diagnostics.AddError(
			"Unexpected Import Identifier",
			fmt.Sprintf("Expected import identifier with format: app_id,volume_internal_id. Got: %q", req.ID),
		)
		return
	}

	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("app"), idParts[0])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("internalid"), idParts[1])...)
}
