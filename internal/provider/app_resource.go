package provider

import (
	"context"
	"errors"
	"fmt"
	"github.com/fly-apps/terraform-provider-fly/graphql"
	"github.com/fly-apps/terraform-provider-fly/internal/provider/modifiers"
	"github.com/fly-apps/terraform-provider-fly/internal/utils"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/superfly/flyctl/api"
	"github.com/vektah/gqlparser/v2/gqlerror"
	"strings"
	"time"
)

var _ resource.ResourceWithConfigure = &flyAppResource{}
var _ resource.ResourceWithImportState = &flyAppResource{}

const ConsulUrlEnv = "FLY_CONSUL_URL"

type appSecret struct {
	Value     types.String `tfsdk:"value"`
	Digest    types.String `tfsdk:"digest"`
	CreatedAt types.String `tfsdk:"created_at"`
}

var secretType = types.ObjectType{
	AttrTypes: map[string]attr.Type{
		"value":      types.StringType,
		"digest":     types.StringType,
		"created_at": types.StringType,
	},
}

type secretMap map[string]appSecret

//var secretMapType = types.MapType{ElemType: secretType}

//type secretMap types.Map

type flyAppResourceData struct {
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

func (d *flyAppResourceData) setSecret(name string, secret appSecret) {
	if d.Secrets == nil {
		d.Secrets = make(secretMap)
	}
	d.Secrets[name] = secret
}

func (d *flyAppResourceData) secretValues() map[string]string {
	values := make(map[string]string, len(d.Secrets))
	for k, v := range d.Secrets {
		values[k] = v.Value.Value
	}
	return values
}

func (d *flyAppResourceData) updateFromApi(a graphql.AppFragment) {
	d.Name = types.String{Value: a.Name}
	d.Org = types.String{Value: a.Organization.Slug}
	d.OrgId = types.String{Value: a.Organization.Id}
	d.AppUrl = types.String{Value: a.AppUrl}
	d.Id = types.String{Value: a.Id}
	if a.PlatformVersion != "" {
		d.Machines = types.Bool{Value: a.PlatformVersion == graphql.PlatformVersionEnumMachines}
	}
	if a.Role != nil && a.Role.GetName() != "" {
		d.RoleId = types.String{Value: a.Role.GetName()}
	} else {
		d.RoleId = types.String{Null: true}
	}
	d.PreferredRegion = types.String{Value: a.Autoscaling.PreferredRegion}
}

func (d *flyAppResourceData) updateSecretsFromApi(a graphql.AppFragment) {
	for k := range d.Secrets {
		d.updateSecretFromApi(k, a)
	}
	hasConsul := false
	for _, s := range a.Secrets {
		if s.Name == ConsulUrlEnv {
			hasConsul = true
		}
	}
	if !hasConsul {
		d.EnableConsul = types.Bool{Value: false}
	}
}

func (d *flyAppResourceData) updateSecretFromApi(name string, a graphql.AppFragment) {
	s := d.Secrets[name]
	for _, as := range a.Secrets {
		if as.Name != name {
			continue
		}
		if as.Digest != s.Digest.Value || as.CreatedAt.Format(time.RFC3339) != s.CreatedAt.Value {
			d.Secrets[name] = appSecret{
				Digest:    types.String{Value: as.Digest},
				CreatedAt: types.String{Value: as.CreatedAt.Format(time.RFC3339)},
				Value:     types.String{Unknown: true},
			}
		}
		return
	}
	// Not found in app, so secret was removed outside of Terraform
	delete(d.Secrets, name)
}

func (r flyAppResource) Metadata(_ context.Context, _ resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = "fly_app"
}

func (r flyAppResource) GetSchema(context.Context) (tfsdk.Schema, diag.Diagnostics) {
	var secretValueUnchanged modifiers.UseStateForUnknownIfFunc = func(ctx context.Context, req tfsdk.ModifyAttributePlanRequest) (ok bool, diags diag.Diagnostics) {
		valuePath := req.AttributePath.ParentPath().AtName("value")
		var stateValue, configValue types.String
		diags.Append(req.State.GetAttribute(ctx, valuePath, &stateValue)...)
		diags.Append(req.Config.GetAttribute(ctx, valuePath, &configValue)...)
		return stateValue.Equal(configValue), diags
	}

	var orgUnchanged modifiers.UseStateForUnknownIfFunc = func(ctx context.Context, req tfsdk.ModifyAttributePlanRequest) (ok bool, diags diag.Diagnostics) {
		var stateOrg, planOrg types.String
		diags.Append(req.State.GetAttribute(ctx, path.Root("org"), &stateOrg)...)
		diags.Append(req.Plan.GetAttribute(ctx, path.Root("org"), &planOrg)...)
		return stateOrg.Equal(planOrg), diags
	}

	return tfsdk.Schema{
		Version:             2,
		MarkdownDescription: "Fly app resource",
		Attributes: map[string]tfsdk.Attribute{
			"name": {
				MarkdownDescription: "Name of application",
				Required:            true,
				Type:                types.StringType,
				PlanModifiers:       []tfsdk.AttributePlanModifier{resource.RequiresReplace()},
			},
			"org": {
				Computed:            true,
				Optional:            true,
				MarkdownDescription: "Optional org slug to operate upon",
				Type:                types.StringType,
			},
			"preferred_region": {
				Optional:      true,
				Computed:      true,
				Type:          types.StringType,
				PlanModifiers: []tfsdk.AttributePlanModifier{resource.RequiresReplace()},
			},
			"machines": {
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Use the machines platform",
				Type:                types.BoolType,
				PlanModifiers: []tfsdk.AttributePlanModifier{
					resource.RequiresReplace(),
					modifiers.Default(types.Bool{Value: false}),
				},
			},
			"secrets": {
				Optional:    true,
				Description: "Secret environment variables. Keys are case sensitive and are used as environment variable names. Does not override existing secrets added outside of Terraform.",
				Attributes: tfsdk.MapNestedAttributes(map[string]tfsdk.Attribute{
					"value": {
						Type:      types.StringType,
						Sensitive: true,
						Required:  true,
					},
					"digest": {
						Type:          types.StringType,
						Computed:      true,
						PlanModifiers: []tfsdk.AttributePlanModifier{modifiers.UseStateForUnknownIf(secretValueUnchanged)},
					},
					"created_at": {
						Type:          types.StringType,
						Computed:      true,
						PlanModifiers: []tfsdk.AttributePlanModifier{modifiers.UseStateForUnknownIf(secretValueUnchanged)},
					},
				}),
			},
			"orgid": {
				Computed:            true,
				MarkdownDescription: "readonly orgid",
				Type:                types.StringType,
				PlanModifiers:       []tfsdk.AttributePlanModifier{modifiers.UseStateForUnknownIf(orgUnchanged)},
			},
			"id": {
				Computed:            true,
				MarkdownDescription: "readonly app id",
				Type:                types.StringType,
				PlanModifiers:       []tfsdk.AttributePlanModifier{resource.UseStateForUnknown()},
			},
			"appurl": {
				Computed:            true,
				MarkdownDescription: "readonly appUrl",
				Type:                types.StringType,
				PlanModifiers:       []tfsdk.AttributePlanModifier{resource.UseStateForUnknown()},
			},
			"role_id": {
				Type:     types.StringType,
				Optional: true,
				PlanModifiers: []tfsdk.AttributePlanModifier{
					modifiers.Default(types.String{Null: true}),
					resource.RequiresReplace(),
				},
			},
			"enable_consul": {
				Type:     types.BoolType,
				Optional: true,
				Computed: true,
				PlanModifiers: []tfsdk.AttributePlanModifier{
					modifiers.Default(types.Bool{Value: false}),
					resource.RequiresReplaceIf(
						func(ctx context.Context, state, config attr.Value, path path.Path) (replace bool, diags diag.Diagnostics) {
							var before, after types.Bool
							diags.Append(tfsdk.ValueAs(ctx, state, &before)...)
							diags.Append(tfsdk.ValueAs(ctx, config, &after)...)
							return before.Equal(types.Bool{Value: true}) && after.Equal(types.Bool{Value: false}), diags
						},
						"consul cannot be disabled after it was enabled",
						"consul cannot be disabled after it was enabled",
					),
				},
			},
			"consul_url": {
				Type:      types.StringType,
				Computed:  true,
				Sensitive: true,
			},
		},
	}, nil
}

func newAppResource() resource.Resource {
	return &flyAppResource{}
}

type flyAppResource struct {
	flyResource
}

func (r flyAppResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data flyAppResourceData

	diags := req.Plan.Get(ctx, &data)
	resp.Diagnostics.Append(diags...)

	if resp.Diagnostics.HasError() {
		return
	}

	org := r.resolveOrganization(ctx, data.Org, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	data.Org.Value = org.Name
	data.OrgId.Value = org.ID

	createAppInput := graphql.CreateAppInput{
		Name:           data.Name.Value,
		OrganizationId: data.OrgId.Value,
		Machines:       data.Machines.Value,
		Runtime:        graphql.RuntimeTypeFirecracker,
		AppRoleId:      data.RoleId.Value,
	}
	if !data.PreferredRegion.IsNull() {
		createAppInput.PreferredRegion = data.PreferredRegion.Value
	}
	mresp, err := graphql.CreateAppMutation(context.Background(), r.gqlClient, createAppInput)
	if err != nil {
		resp.Diagnostics.AddError("Create app failed", err.Error())
		return
	}
	data.updateFromApi(mresp.CreateApp.App)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if data.EnableConsul.Equal(types.Bool{Value: true}) {
		resp.Diagnostics.Append(r.enableConsul(ctx, &data, &resp.State)...)
		if resp.Diagnostics.HasError() {
			return
		}
	} else {
		data.ConsulUrl = types.String{Null: true}
	}

	if len(data.Secrets) > 0 {
		r.setSecrets(ctx, data, data.Secrets, &resp.Diagnostics)
		if resp.Diagnostics.HasError() {
			return
		}
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r flyAppResource) enableConsul(ctx context.Context, data *flyAppResourceData, state *tfsdk.State) (diags diag.Diagnostics) {
	consulPayload, err := r.apiClient.EnablePostgresConsul(ctx, data.Name.Value)
	if err != nil {
		diags.AddError("EnablePostgresConsul failed", err.Error())
		return
	}

	_, err = graphql.SetSecrets(ctx, r.gqlClient, graphql.SetSecretsInput{
		AppId: data.Name.Value,
		Secrets: []graphql.SecretInput{
			{Key: ConsulUrlEnv, Value: consulPayload.ConsulURL},
		},
	})
	var errList gqlerror.List
	if errors.As(err, &errList) && len(errList) == 1 && strings.Contains(strings.ToLower(errList[0].Message), "no change detected") {
		// ignore
	} else if err != nil {
		diags.AddError("Failed setting FLY_CONSUL_URL secret", err.Error())
		return
	}

	data.EnableConsul = types.Bool{Value: true}
	data.ConsulUrl = types.String{Value: consulPayload.ConsulURL}
	diags.Append(state.Set(ctx, &data)...)
	return
}

func (r flyAppResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state flyAppResourceData

	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)

	if resp.Diagnostics.HasError() {
		return
	}

	query, err := graphql.GetApp(context.Background(), r.gqlClient, state.Name.Value)
	var errList gqlerror.List
	if errors.As(err, &errList) {
		for _, err := range errList {
			if strings.Contains(strings.ToLower(err.Message), "could not find app") {
				resp.State.RemoveResource(ctx)
				return
			} else {
				resp.Diagnostics.AddError(err.Message, err.Path.String())
			}
		}
	} else if err != nil {
		resp.Diagnostics.AddError("Read: query failed", err.Error())
	}
	if resp.Diagnostics.HasError() {
		return
	}

	state.updateFromApi(query.App)
	state.updateSecretsFromApi(query.App)

	diags = resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}

func (r flyAppResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state flyAppResourceData
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if plan.Org != state.Org {
		org := r.resolveOrganization(ctx, plan.Org, &resp.Diagnostics)
		if resp.Diagnostics.HasError() {
			return
		}
		app, err := r.apiClient.MoveApp(ctx, state.Name.Value, org.ID)
		if err != nil {
			resp.Diagnostics.AddError("Error moving app", err.Error())
		} else {
			state.Org = types.String{Value: app.Organization.Slug}
			state.Id = types.String{Value: app.Organization.ID}
			resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
		}
		if resp.Diagnostics.HasError() {
			return
		}
	}

	state.Machines = plan.Machines

	if plan.EnableConsul.Equal(types.Bool{Value: true}) {
		resp.Diagnostics.Append(r.enableConsul(ctx, &state, &resp.State)...)
		if resp.Diagnostics.HasError() {
			return
		}
	} else {
		state.EnableConsul = plan.EnableConsul
		state.ConsulUrl = types.String{Null: true}
	}

	// Unset secrets that were removed from config
	var removedSecrets []string
	for k := range state.Secrets {
		if _, ok := plan.Secrets[k]; !ok {
			removedSecrets = append(removedSecrets, k)
			delete(state.Secrets, k)
		}
	}
	if len(removedSecrets) > 0 {
		_, err := r.apiClient.UnsetSecrets(ctx, state.Name.Value, removedSecrets)
		if err != nil {
			resp.Diagnostics.AddError("UnsetSecrets failed", err.Error())
		} else {
			resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
		}
		if resp.Diagnostics.HasError() {
			return
		}
	}

	// Set secrets that were changed or newly appeared in config
	newSecrets := make(secretMap)
	for k, ps := range plan.Secrets {
		if s, ok := state.Secrets[k]; !ok || !s.Value.Equal(ps.Value) {
			newSecrets[k] = ps
		}
	}
	if len(newSecrets) > 0 {
		r.setSecrets(ctx, state, newSecrets, &resp.Diagnostics)
		for k, s := range newSecrets {
			state.setSecret(k, s)
		}
		if resp.Diagnostics.HasError() {
			return
		}
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}

func (r flyAppResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data flyAppResourceData

	diags := req.State.Get(ctx, &data)
	resp.Diagnostics.Append(diags...)

	err := r.apiClient.DeleteApp(ctx, data.Name.Value)
	var errList gqlerror.List
	if errors.As(err, &errList) {
		for _, err := range errList {
			resp.Diagnostics.AddError(err.Message, err.Path.String())
		}
	} else if err != nil {
		resp.Diagnostics.AddError("Delete app failed", err.Error())
	}
}

func (r flyAppResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("name"), req, resp)
}

func (r flyAppResource) resolveOrganization(ctx context.Context, slug types.String, diags *diag.Diagnostics) (org *api.Organization) {
	var err error
	if slug.IsUnknown() || slug.IsNull() {
		org, err = utils.GetDefaultOrg(ctx, r.apiClient)
	} else {
		org, err = r.apiClient.GetOrganizationBySlug(ctx, slug.Value)
	}
	if err != nil {
		diags.AddError(fmt.Sprintf("Could not resolve organization %s", slug.Value), err.Error())
	}
	return
}

// setSecrets sets the app's secrets specified in the secret map `secrets` and
// writes back `digest` and `createdAt` attributes.
func (r flyAppResource) setSecrets(ctx context.Context, data flyAppResourceData, secrets secretMap, diags *diag.Diagnostics) {
	inputs := make([]graphql.SecretInput, len(secrets))
	i := 0
	for k, v := range secrets {
		inputs[i] = graphql.SecretInput{
			Key:   k,
			Value: v.Value.Value,
		}
		i++
	}

	resp, err := graphql.SetSecrets(ctx, r.gqlClient, graphql.SetSecretsInput{
		AppId:      data.Name.Value,
		Secrets:    inputs,
		ReplaceAll: false,
	})
	if err != nil {
		var errList gqlerror.List
		if errors.As(err, &errList) && len(errList) == 1 && strings.Contains(errList[0].Message, "No change detected") {
			diags.AddWarning("SetSecrets was no-op", err.Error())
			// Secrets may have been added outside of Terraform. To prevent unknown value errors, we need to fetch
			// digest and createdAt attributes for secrets that don't have those values missing in state
			diags.AddWarning("State may have drifted", "Secrets may have been added or changed outside of Terraform. Refreshing secret state, you may need to re-apply your Terraform config.")
			r.getSecretComputedValues(ctx, data.Name.Value, secrets, diags)
		} else {
			diags.AddError("SetSecrets errored", err.Error())
		}
		return
	}

	for _, s := range resp.SetSecrets.App.Secrets {
		if _, ok := secrets[s.Name]; !ok {
			continue
		}
		secrets[s.Name] = appSecret{
			Value:     secrets[s.Name].Value,
			Digest:    types.String{Value: s.Digest},
			CreatedAt: types.String{Value: s.CreatedAt.Format(time.RFC3339)},
		}
	}
}

func (r flyAppResource) getSecretComputedValues(ctx context.Context, appName string, secrets secretMap, diags *diag.Diagnostics) {
	apiSecrets, err := r.apiClient.GetAppSecrets(ctx, appName)
	if err != nil {
		diags.AddError("GetSecrets failed", err.Error())
	}
	for _, as := range apiSecrets {
		if s, ok := secrets[as.Name]; ok {
			secrets[as.Name] = appSecret{
				Value:     s.Value,
				Digest:    types.String{Value: as.Digest},
				CreatedAt: types.String{Value: as.CreatedAt.Format(time.RFC3339)},
			}
		}
	}
}
