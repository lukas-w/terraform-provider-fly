package provider

import (
	"context"
	"fmt"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	tfsdkprovider "github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/lukas-w/terraform-provider-fly/internal/flyapi"
	"github.com/superfly/fly-go"
	"github.com/superfly/fly-go/flaps"
	"os"
)

var _ tfsdkprovider.Provider = &provider{}

type provider struct {
	version string
	token   string
}

type providerApi struct {
	c         *flyapi.Clients
	gqlClient flyapi.GqlClient
	apiClient fly.Client
}

func (c *providerApi) configure(providerData any, diags *diag.Diagnostics) {
	if providerData == nil {
		return
	}

	if p, ok := providerData.(*providerApi); ok {
		*c = *p
	} else {
		diags.AddError(
			"Unexpected Provider Instance Type",
			fmt.Sprintf("While creating the data source or resource, an unexpected clients type (%T) was received. This is always a bug in the clients code and should be reported to the clients developers.", p),
		)
	}
}

func (p *providerApi) getFlapsClient(ctx context.Context, app string, diags *diag.Diagnostics) *flaps.Client {
	client, err := p.c.GetFlapsClient(ctx, app)
	if err != nil {
		diags.AddError("Error getting flaps client", err.Error())
	}
	return client
}

type providerData struct {
	FlyToken        types.String `tfsdk:"fly_api_token"`
	FlyHttpEndpoint types.String `tfsdk:"fly_http_endpoint"`
}

func (p *provider) Configure(ctx context.Context, req tfsdkprovider.ConfigureRequest, resp *tfsdkprovider.ConfigureResponse) {
	var data providerData
	diags := req.Config.Get(ctx, &data)
	resp.Diagnostics.Append(diags...)

	if resp.Diagnostics.HasError() {
		return
	}

	var token string
	if data.FlyToken.IsUnknown() {
		resp.Diagnostics.AddWarning(
			"Unable to create gqlClient",
			"Cannot use unknown value as token",
		)
		return
	}
	if data.FlyToken.IsNull() || data.FlyToken.IsUnknown() {
		token = os.Getenv("FLY_API_TOKEN")
	} else {
		token = data.FlyToken.ValueString()
	}
	if token == "" {
		resp.Diagnostics.AddError(
			"Unable to find token",
			"token cannot be an empty string",
		)
		return
	}

	p.token = token

	enableTracing := false
	_, ok := os.LookupEnv("DEBUG")
	if ok {
		enableTracing = true
		resp.Diagnostics.AddWarning("Debug mode enabled", "Debug mode enabled, this will add the Fly-Force-Trace header to all graphql requests")
	}

	var clients providerApi
	api := flyapi.New(ctx, flyapi.Options{
		AccessToken:      token,
		Version:          p.version,
		EnableDebugTrace: enableTracing,
	})
	clients.c = api
	clients.apiClient = api.FlyGo
	clients.gqlClient = api.Gql

	fly.SetErrorLog(true)

	resp.ResourceData = &clients
	resp.DataSourceData = &clients
}

func (p *provider) Resources(context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		newAppResource,
		newDeployTokenResource,
		newFlySecretResource,
		newFlyVolumeResource,
		newFlyIpResource,
		newFlyCertResource,
		newFlyMachineResource,
		newOrganizationResource,
		newRedisResource,
		newFlyWireguardTokenResource,
	}
}

func (p *provider) DataSources(context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{
		newAppDataSource,
		newCertDataSource,
		newIpDataSource,
		newOrganizationDataSource,
		NewVolumeDataSource,
	}
}

func (p *provider) Metadata(_ context.Context, _ tfsdkprovider.MetadataRequest, resp *tfsdkprovider.MetadataResponse) {
	resp.TypeName = "fly"
}

func (p *provider) Schema(_ context.Context, _ tfsdkprovider.SchemaRequest, resp *tfsdkprovider.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"fly_api_token": schema.StringAttribute{
				MarkdownDescription: "fly.io api token. If not set checks env for FLY_API_TOKEN",
				Optional:            true,
			},
			"fly_http_endpoint": schema.StringAttribute{
				MarkdownDescription: "Where the clients should look to find the fly http endpoint",
				Optional:            true,
			},
		},
	}
}

func New(version string) func() tfsdkprovider.Provider {
	return func() tfsdkprovider.Provider {
		return &provider{
			version: version,
		}
	}
}
