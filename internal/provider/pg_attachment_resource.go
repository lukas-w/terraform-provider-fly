package provider

import (
	"context"
	"github.com/fly-apps/terraform-provider-fly/internal/provider/modifiers"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/superfly/flyctl/api"
)

type pgAttachmentResourceModel struct {
	App          string       `tfsdk:"app"`
	PostgresApp  string       `tfsdk:"postgres_app"`
	DatabaseName string       `tfsdk:"database_name"`
	DatabaseUser string       `tfsdk:"database_user"`
	VariableName string       `tfsdk:"variable_name"`
	Id           types.String `tfsdk:"id"`
}

type pgAttachmentResource struct {
	flyResource
}

func newPgAttachmentResource() resource.Resource {
	return &pgAttachmentResource{}
}

func (p pgAttachmentResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = "fly_pg_attachment"
}

func (p pgAttachmentResource) GetSchema(ctx context.Context) (tfsdk.Schema, diag.Diagnostics) {
	return tfsdk.Schema{
		Attributes: map[string]tfsdk.Attribute{
			"id": {
				Type:     types.StringType,
				Computed: true,
			},
			"app": {
				Type:     types.StringType,
				Required: true,
			},
			"postgres_app": {
				Type:     types.StringType,
				Required: true,
			},
			"database_name": {
				Type:     types.StringType,
				Required: true,
			},
			"database_user": {
				Type:     types.StringType,
				Required: true,
			},
			"variable_name": {
				Type:          types.StringType,
				Optional:      true,
				Computed:      true,
				PlanModifiers: []tfsdk.AttributePlanModifier{modifiers.Default(types.String{Value: "DATABASE_URL"})},
			},
		},
	}, nil
}

func (p pgAttachmentResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan pgAttachmentResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)

	r, err := p.apiClient.AttachPostgresCluster(ctx, api.AttachPostgresClusterInput{
		AppID:                plan.App,
		PostgresClusterAppID: plan.PostgresApp,
		DatabaseName:         &plan.DatabaseName,
		DatabaseUser:         &plan.DatabaseUser,
		VariableName:         &plan.VariableName,
		ManualEntry:          true,
	})
	if err != nil {
		resp.Diagnostics.AddError("Error in API call", err.Error())
		return
	}
	plan.VariableName = r.EnvironmentVariableName
	plan.App = r.App.Name
	plan.PostgresApp = r.PostgresClusterApp.Name

	attachments, err := p.apiClient.ListPostgresClusterAttachments(ctx, plan.App, plan.PostgresApp)
	attachment := findAttachment(attachments, plan)
	if attachment == nil {
		resp.Diagnostics.AddError("Can't find the attachment we just created", err.Error())
		return
	}
	plan.Id = types.String{Value: attachment.ID}

	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func findAttachment(attachments []*api.PostgresClusterAttachment, data pgAttachmentResourceModel) *api.PostgresClusterAttachment {
	for _, a := range attachments {
		if a.DatabaseName == data.DatabaseName && a.DatabaseUser == data.DatabaseUser && a.EnvironmentVariableName == data.VariableName {
			return a
		}
	}
	return nil
}

func (p pgAttachmentResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state pgAttachmentResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)

	attachments, err := p.apiClient.ListPostgresClusterAttachments(ctx, state.App, state.PostgresApp)
	if err != nil {
		resp.Diagnostics.AddError("Error in API call", err.Error())
		return
	}
	for _, a := range attachments {
		if a.ID == state.Id.Value {
			return
		}
	}
	resp.State.RemoveResource(ctx)
}

func (p pgAttachmentResource) Update(context.Context, resource.UpdateRequest, *resource.UpdateResponse) {
}

func (p pgAttachmentResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state pgAttachmentResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)

	err := p.apiClient.DetachPostgresCluster(ctx, api.DetachPostgresClusterInput{
		AppID:                       state.App,
		PostgresClusterId:           state.PostgresApp,
		PostgresClusterAttachmentId: state.Id.Value,
	})
	if err != nil {
		resp.Diagnostics.AddError("Error in API call", err.Error())
		return
	}
}
