package provider

import (
	"context"
	"fmt"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/superfly/fly-go"
	"strings"
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
	d.Id = types.String{Value: v.ID}
	d.Name = types.String{Value: v.Name}
	d.Size = types.Int64{Value: int64(v.SizeGb)}
	d.Region = types.String{Value: v.Region}
	d.Encrypted = types.Bool{Value: v.Encrypted}
}

func (vr flyVolumeResource) Metadata(_ context.Context, _ resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = "fly_volume"
}

func (vr flyVolumeResource) GetSchema(context.Context) (tfsdk.Schema, diag.Diagnostics) {
	return tfsdk.Schema{
		MarkdownDescription: "Fly volume resource",
		Attributes: map[string]tfsdk.Attribute{
			"id": {
				MarkdownDescription: "ID of volume",
				Type:                types.StringType,
				Computed:            true,
			},
			"app": {
				MarkdownDescription: "Name of app to attach",
				Required:            true,
				Type:                types.StringType,
				PlanModifiers:       []tfsdk.AttributePlanModifier{resource.RequiresReplace()},
			},
			"size": {
				MarkdownDescription: "Size of volume in gb",
				Required:            true,
				Type:                types.Int64Type,
				PlanModifiers: []tfsdk.AttributePlanModifier{
					resource.RequiresReplaceIf(func(ctx context.Context, state, config attr.Value, path path.Path) (replace bool, d diag.Diagnostics) {
						var size, newSize int64
						d.Append(tfsdk.ValueAs(ctx, state, &size)...)
						d.Append(tfsdk.ValueAs(ctx, config, &newSize)...)
						replace = newSize < size
						return
					}, "volumes cannot be made smaller", "volumes cannot be made smaller"),
				},
			},
			"name": {
				MarkdownDescription: "name",
				Type:                types.StringType,
				Required:            true,
				PlanModifiers:       []tfsdk.AttributePlanModifier{resource.RequiresReplace()},
			},
			"region": {
				MarkdownDescription: "region",
				Type:                types.StringType,
				Required:            true,
				PlanModifiers:       []tfsdk.AttributePlanModifier{resource.RequiresReplace()},
			},
			"encrypted": {
				MarkdownDescription: "Encrypt the volume contents. Defaults to `true`",
				Type:                types.BoolType,
				Optional:            true,
				Computed:            true,
			},
		},
	}, nil
}

func (vr flyVolumeResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data flyVolumeResourceData

	diags := req.Plan.Get(ctx, &data)
	resp.Diagnostics.Append(diags...)

	client := vr.getFlapsClient(ctx, data.Appid.Value, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	sizeGb := int(data.Size.Value)
	encrypted := data.Encrypted.IsNull() || data.Encrypted.IsUnknown() || data.Encrypted.Value
	v, err := client.CreateVolume(ctx, fly.CreateVolumeRequest{
		Name:      data.Name.Value,
		Region:    data.Region.Value,
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

	client := vr.getFlapsClient(ctx, data.Appid.Value, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	v, err := client.GetVolume(ctx, data.Id.Value)
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

	client := vr.getFlapsClient(ctx, state.Appid.Value, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	if plan.Size.Value > state.Size.Value {
		v, needsRestart, err := client.ExtendVolume(ctx, state.Id.Value, int(plan.Size.Value))
		if err != nil {
			resp.Diagnostics.AddError("error extending volume", err.Error())
			return
		}
		if needsRestart {
			resp.Diagnostics.AddWarning(
				"app needs restart",
				fmt.Sprintf("app %s needs to be restarted after extending volume %s with ID %s", state.Appid.Value, state.Name.Value, state.Id.Value),
			)
		}
		state.fromApiVolume(*v)
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (vr flyVolumeResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data flyVolumeResourceData
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	client := vr.getFlapsClient(ctx, data.Appid.Value, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	_, err := client.DeleteVolume(ctx, data.Id.Value)
	if err != nil {
		resp.Diagnostics.AddError("Delete volume failed", err.Error())
		return
	}
}

func (vr flyVolumeResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
