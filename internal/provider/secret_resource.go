package provider

import (
	"context"
	"github.com/fly-apps/terraform-provider-fly/graphql"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	tfsdkprovider "github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
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

type flySecretResourceType struct{}

func (s flySecretResourceType) GetSchema(ctx context.Context) (tfsdk.Schema, diag.Diagnostics) {
	return tfsdk.Schema{
		Attributes: map[string]tfsdk.Attribute{
			"app": {
				Type:     types.StringType,
				Required: true,
			},
			"name": {
				Type:     types.StringType,
				Required: true,
			},
			"value": {
				Type:      types.StringType,
				Required:  true,
				Sensitive: true,
			},
			"digest": {
				Type:     types.StringType,
				Computed: true,
			},
			"created_at": {
				Type:     types.StringType,
				Computed: true,
			},
		},
	}, nil
}

func (s flySecretResourceType) NewResource(ctx context.Context, p tfsdkprovider.Provider) (resource.Resource, diag.Diagnostics) {
	provider, diags := convertProviderType(p)
	return secretResource{
		provider: provider,
	}, diags
}

type secretResource struct {
	provider provider
}

func (r secretResource) Create(ctx context.Context, request resource.CreateRequest, response *resource.CreateResponse) {
	var data secretResourceModel
	response.Diagnostics.Append(request.Plan.Get(ctx, &data)...)

	s := r.setSecret(ctx, data.App.Value, data.Name.Value, data.Value.Value, &response.Diagnostics)
	if response.Diagnostics.HasError() {
		return
	}
	data.Digest = types.String{Value: s.Digest}
	data.CreatedAt = types.String{Value: s.CreatedAt.Format(time.RFC3339)}

	response.Diagnostics.Append(response.State.Set(ctx, data)...)
}

func (r secretResource) Read(ctx context.Context, request resource.ReadRequest, response *resource.ReadResponse) {
	var data secretResourceModel
	response.Diagnostics.Append(request.State.Get(ctx, &data)...)
	s := r.getSecret(ctx, data.App.Value, data.Name.Value, &response.Diagnostics)
	if response.Diagnostics.HasError() {
		return
	}
	if s == nil {
		response.State.RemoveResource(ctx)
		return
	}

	if data.CreatedAt.Value != s.CreatedAt.Format(time.RFC3339) || data.Digest.Value != s.Digest {
		data.Value = types.String{Unknown: true}
	}
	data.Digest = types.String{Value: s.Digest}
	data.CreatedAt = types.String{Value: s.CreatedAt.Format(time.RFC3339)}
	response.Diagnostics.Append(response.State.Set(ctx, data)...)
}

func (r secretResource) Update(ctx context.Context, request resource.UpdateRequest, response *resource.UpdateResponse) {
	var data secretResourceModel
	response.Diagnostics.Append(request.Plan.Get(ctx, &data)...)

	s := r.setSecret(ctx, data.App.Value, data.Name.Value, data.Value.Value, &response.Diagnostics)
	if response.Diagnostics.HasError() {
		return
	}

	data.Digest = types.String{Value: s.Digest}
	data.CreatedAt = types.String{Value: s.CreatedAt.Format(time.RFC3339)}
	response.Diagnostics.Append(response.State.Set(ctx, data)...)
}

func (r secretResource) Delete(ctx context.Context, request resource.DeleteRequest, response *resource.DeleteResponse) {
	var data secretResourceModel
	response.Diagnostics.Append(request.State.Get(ctx, &data)...)

	resp, err := graphql.UnsetSecrets(ctx, *r.provider.client, data.App.Value, []string{data.Name.Value})
	if response.Diagnostics.HasError() {
		return
	}
	if err != nil {
		response.Diagnostics.AddError("error unsetting secret", err.Error())
		return
	}
	for _, secret := range resp.UnsetSecrets.App.Secrets {
		if secret.Name == data.Name.Value {
			response.Diagnostics.AddError("error unsetting secret", "api call successful but secret is still there")
			return
		}
	}
}

func (r secretResource) getSecret(ctx context.Context, app, name string, diags *diag.Diagnostics) *graphql.SecretFragment {
	resp, err := graphql.GetSecrets(ctx, *r.provider.client, app)
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
	resp, err := graphql.SetSecrets(ctx, *r.provider.client, graphql.SetSecretsInput{
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
