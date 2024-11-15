package provider

import (
	"context"
	"github.com/fly-apps/terraform-provider-fly/graphql"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"time"
)

type secretResourceModel struct {
	App       types.String `tfsdk:"app"`
	Name      types.String `tfsdk:"name"`
	Value     types.String `tfsdk:"value"`
	Digest    types.String `tfsdk:"digest"`
	CreatedAt types.String `tfsdk:"created_at"`
}

type secretResource struct {
	flyResource
}

func (r secretResource) Metadata(_ context.Context, _ resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = "fly_secret"
}

func (r secretResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"app": schema.StringAttribute{
				Required: true,
			},
			"name": schema.StringAttribute{
				Required: true,
			},
			"value": schema.StringAttribute{
				Required:  true,
				Sensitive: true,
			},
			"digest": schema.StringAttribute{
				Computed: true,
			},
			"created_at": schema.StringAttribute{
				Computed: true,
			},
		},
	}
}

func newFlySecretResource() resource.Resource {
	return &secretResource{}
}

func (r secretResource) Create(ctx context.Context, request resource.CreateRequest, response *resource.CreateResponse) {
	var data secretResourceModel
	response.Diagnostics.Append(request.Plan.Get(ctx, &data)...)

	s := r.setSecret(ctx, data.App.ValueString(), data.Name.ValueString(), data.Value.ValueString(), &response.Diagnostics)
	if response.Diagnostics.HasError() {
		return
	}
	data.Digest = types.StringValue(s.Digest)
	data.CreatedAt = types.StringValue(s.CreatedAt.Format(time.RFC3339))

	response.Diagnostics.Append(response.State.Set(ctx, data)...)
}

func (r secretResource) Read(ctx context.Context, request resource.ReadRequest, response *resource.ReadResponse) {
	var data secretResourceModel
	response.Diagnostics.Append(request.State.Get(ctx, &data)...)
	s := r.getSecret(ctx, data.App.ValueString(), data.Name.ValueString(), &response.Diagnostics)
	if response.Diagnostics.HasError() {
		return
	}
	if s == nil {
		response.State.RemoveResource(ctx)
		return
	}

	if data.CreatedAt.ValueString() != s.CreatedAt.Format(time.RFC3339) || data.Digest.ValueString() != s.Digest {
		data.Value = types.StringUnknown()
	}
	data.Digest = types.StringValue(s.Digest)
	data.CreatedAt = types.StringValue(s.CreatedAt.Format(time.RFC3339))
	response.Diagnostics.Append(response.State.Set(ctx, data)...)
}

func (r secretResource) Update(ctx context.Context, request resource.UpdateRequest, response *resource.UpdateResponse) {
	var data secretResourceModel
	response.Diagnostics.Append(request.Plan.Get(ctx, &data)...)

	s := r.setSecret(ctx, data.App.ValueString(), data.Name.ValueString(), data.Value.ValueString(), &response.Diagnostics)
	if response.Diagnostics.HasError() {
		return
	}

	data.Digest = types.StringValue(s.Digest)
	data.CreatedAt = types.StringValue(s.CreatedAt.Format(time.RFC3339))
	response.Diagnostics.Append(response.State.Set(ctx, data)...)
}

func (r secretResource) Delete(ctx context.Context, request resource.DeleteRequest, response *resource.DeleteResponse) {
	var data secretResourceModel
	response.Diagnostics.Append(request.State.Get(ctx, &data)...)

	resp, err := graphql.UnsetSecrets(ctx, r.gqlClient, data.App.ValueString(), []string{data.Name.ValueString()})
	if response.Diagnostics.HasError() {
		return
	}
	if err != nil {
		response.Diagnostics.AddError("error unsetting secret", err.Error())
		return
	}
	for _, secret := range resp.UnsetSecrets.App.Secrets {
		if secret.Name == data.Name.ValueString() {
			response.Diagnostics.AddError("error unsetting secret", "api call successful but secret is still there")
			return
		}
	}
}

func (r secretResource) getSecret(ctx context.Context, app, name string, diags *diag.Diagnostics) *graphql.SecretFragment {
	resp, err := graphql.GetSecrets(ctx, r.gqlClient, app)
	if err != nil {
		diags.AddError("error getting secrets", err.Error())
	}
	for _, secret := range resp.App.Secrets {
		if secret.Name == name {
			return &secret
		}
	}
	return nil
}

func (r secretResource) setSecret(ctx context.Context, app, name, value string, diags *diag.Diagnostics) *graphql.SecretFragment {
	resp, err := graphql.SetSecrets(ctx, r.gqlClient, graphql.SetSecretsInput{
		AppId: app,
		Secrets: []graphql.SecretInput{{
			Key:   name,
			Value: value,
		}},
	})
	if err != nil {
		diags.AddError("error setting secrets", err.Error())
	}
	for _, secret := range resp.SetSecrets.App.Secrets {
		if secret.Name == name {
			return &secret
		}
	}
	return nil
}
