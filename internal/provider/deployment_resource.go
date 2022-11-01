package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/fly-apps/terraform-provider-fly/internal/provider/validators"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/superfly/flyctl/api"
	"strings"
	"time"
)

type deploymentResourceModel struct {
	App          types.String `tfsdk:"app"`
	Image        types.String `tfsdk:"image"`
	Strategy     types.String `tfsdk:"strategy"`
	Definition   types.String `tfsdk:"definition"`
	ID           types.String `tfsdk:"id"`
	EvaluationID types.String `tfsdk:"evaluation_id"`
}

func (r *deploymentResourceModel) fromApiDeploymentStatus(ds api.DeploymentStatus) {
	r.ID = types.String{Value: ds.ID}
}

func (r *deploymentResourceModel) fromApiRelease(release api.Release) {
	r.EvaluationID = types.String{Value: release.EvaluationID}
}

type deploymentResource struct {
	flyResource
}

func newFlyDeploymentResource() resource.Resource {
	return &deploymentResource{}
}

func (r deploymentResource) Metadata(_ context.Context, _ resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = "fly_deployment"
}

func (r deploymentResource) GetSchema(ctx context.Context) (tfsdk.Schema, diag.Diagnostics) {
	return tfsdk.Schema{
		Attributes: map[string]tfsdk.Attribute{
			"app": {
				Type:          types.StringType,
				PlanModifiers: []tfsdk.AttributePlanModifier{resource.RequiresReplace()},
			},
			"image": {
				Type:          types.StringType,
				PlanModifiers: []tfsdk.AttributePlanModifier{resource.RequiresReplace()},
			},
			"strategy": {
				Type:     types.StringType,
				Optional: true,
				//Computed: true,
			},
			"definition": {
				Type:          types.StringType,
				Description:   "JSON-encoded app definition",
				Validators:    []tfsdk.AttributeValidator{validators.JSON()},
				PlanModifiers: []tfsdk.AttributePlanModifier{resource.RequiresReplace()},
			},
			"id": {
				Type:     types.StringType,
				Computed: true,
			},
			"evaluation_id": {
				Type:     types.StringType,
				Computed: true,
			},
		},
		//Blocks: map[string]tfsdk.Block{
		//	"service": {
		//		NestingMode: tfsdk.BlockNestingModeList,
		//		Attributes: map[string]tfsdk.Attribute{
		//			"description": {
		//				Type: types.StringType,
		//			},
		//			"protocol": {
		//				Type:     types.StringType,
		//				Optional: true,
		//				Computed: true,
		//			},
		//			"internal_port": {
		//				Type:     types.Int64Type,
		//				Optional: true,
		//				Computed: true,
		//			},
		//		},
		//		Blocks: map[string]tfsdk.Block{
		//			"port": {
		//				NestingMode: tfsdk.BlockNestingModeList,
		//				Attributes: map[string]tfsdk.Attribute{
		//					"port": {
		//						Type:     types.Int64Type,
		//						Required: true,
		//					},
		//					"handlers": {
		//						Type: types.ListType{ElemType: types.StringType},
		//					},
		//					"tls_options": {
		//						Type: types
		//					},
		//				},
		//			},
		//			"check": {
		//				NestingMode: tfsdk.BlockNestingModeList,
		//			},
		//			"concurrency": {
		//				NestingMode: tfsdk.BlockNestingModeSingle,
		//				Attributes: map[string]tfsdk.Attribute{
		//					"hard_limit": {
		//						Type: types.StringType,
		//					},
		//					"soft_limit": {
		//						Type: types.StringType,
		//					},
		//				},
		//			},
		//		},
		//	},
		//},
	}, nil
}

func (r deploymentResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data deploymentResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var definition api.Definition
	if err := json.Unmarshal([]byte(data.Definition.Value), &definition); err != nil {
		resp.Diagnostics.AddError("Could not unmarshal definition JSON", err.Error())
	}

	input := api.DeployImageInput{
		AppID:      data.App.Value,
		Image:      data.Image.Value,
		Definition: &definition,
	}
	if data.Strategy.IsNull() || data.Strategy.IsUnknown() {
		input.Strategy = &data.Strategy.Value
	}

	release, releaseCommand, err := r.apiClient.DeployImage(ctx, input)

	if releaseCommand != nil {
		//cmdfmt.PrintBegin(cmdCtx.Out, "Release command")
		tflog.Debug(ctx, fmt.Sprintf("Release command: %s", releaseCommand.Command))
		//fmt.Printf("Command: %s\n", releaseCommand.Command)

		resp.Diagnostics.Append(r.watchReleaseCommand(ctx, releaseCommand.ID)...)
		if resp.Diagnostics.HasError() {
			return
		}

		release, err = r.apiClient.GetAppRelease(ctx, input.AppID, release.ID)
		if err != nil {
			resp.Diagnostics.AddError("failed getting app release", err.Error())
			return
		}
	}
	data.fromApiRelease(*release)

	if strings.ToUpper(release.DeploymentStrategy) == "IMMEDIATE" {
		data.ID = types.String{Null: true}
		data.EvaluationID = types.String{Null: true}
	} else {
		var ds api.DeploymentStatus
		resp.Diagnostics.Append(r.watchDeployment(ctx, input.AppID, release.EvaluationID, &ds)...)
		if resp.Diagnostics.HasError() {
			return
		}
		data.fromApiDeploymentStatus(ds)
	}

	resp.State.Set(ctx, data)
}

func (r deploymentResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data deploymentResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	ds, err := r.apiClient.GetDeploymentStatus(ctx, data.App.Value, data.ID.String(), data.EvaluationID.String())
	if err != nil {
		resp.Diagnostics.AddError("error getting deployment status", err.Error())
		return
	}

	data.fromApiDeploymentStatus(*ds)
	resp.State.Set(ctx, data)
}

func (r deploymentResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
}

func (r deploymentResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
}

func (r deploymentResource) watchReleaseCommand(ctx context.Context, id string) (diag diag.Diagnostics) {
	for {
		rc, err := r.apiClient.GetReleaseCommand(ctx, id)
		if err != nil {
			diag.AddError("error getting release command status", err.Error())
			return
		}

		if rc.InProgress {
			time.Sleep(500 * time.Millisecond)
		} else {
			if rc.Failed {
				diag.AddError(
					"release command failed",
					fmt.Sprintf("id %v, evaluation id %v, exit code %v", rc.ID, rc.EvaluationID, rc.ExitCode),
				)
			} else {
				return nil
			}
		}
	}
}

func (r deploymentResource) watchDeployment(ctx context.Context, app, evaluationID string, ds *api.DeploymentStatus) (diag diag.Diagnostics) {
	pollInterval := 750 * time.Millisecond
	deploymentID := ""
	for {
		select {
		case <-time.After(pollInterval):
			var err error
			ds, err = r.apiClient.GetDeploymentStatus(ctx, app, deploymentID, evaluationID)
			if err != nil {
				diag.AddError("error getting deployment status", err.Error())
			}
			deploymentID = ds.ID
			if ds.InProgress {
				continue
			}
			if !ds.Successful {
				diag.AddError("deployment was unsuccessful", "")
			}
			return
		case <-ctx.Done():
			if ctx.Err() != nil {
				diag.AddError("context error while watching deployment", ctx.Err().Error())
			}
			return
		}
	}
}
