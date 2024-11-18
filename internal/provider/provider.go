package provider

import (
	"context"
	"fmt"
	"github.com/Khan/genqlient/graphql"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	tfsdkprovider "github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/superfly/fly-go"
	"github.com/superfly/fly-go/flaps"
	"github.com/superfly/fly-go/tokens"
	"os"
)

var _ tfsdkprovider.Provider = &provider{}

const baseUrl = "https://api.fly.io"

type gqlClient graphql.Client

type logger struct {
	context.Context
}

func (l logger) Debug(v ...interface{}) {
	tflog.Debug(l.Context, fmt.Sprint(v...))
}

func (l logger) Debugf(format string, v ...interface{}) {
	tflog.Debug(l.Context, fmt.Sprintf(format, v...))
}

type provider struct {
	version string
	token   string
}

type providerApi struct {
	httpEndpoint string
	gqlClient    gqlClient
	apiClient    fly.Client
	flapsClients map[string]*flaps.Client
	tokens       *tokens.Tokens
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
	if p.flapsClients == nil {
		p.flapsClients = make(map[string]*flaps.Client)
	}

	if client, ok := p.flapsClients[app]; ok {
		return client
	}

	appCompact, err := p.apiClient.GetAppCompact(ctx, app)
	if err != nil {
		diags.AddError("Failed to get app compact", err.Error())
		return nil
	}
	client, err := flaps.NewWithOptions(ctx, flaps.NewClientOpts{
		AppName:    app,
		AppCompact: appCompact,
		Tokens:     p.tokens,
		//OrgSlug:     appCompact.Organization.Slug,
	})
	if err != nil {
		diags.AddError("Failed to create flaps client", err.Error())
		return nil
	}

	p.flapsClients[app] = client
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

	apiBaseUrl, exists := os.LookupEnv("FLY_HTTP_ENDPOINT")
	httpEndpoint := "127.0.0.1:4280"
	if !data.FlyHttpEndpoint.IsNull() && !data.FlyHttpEndpoint.IsUnknown() {
		httpEndpoint = data.FlyHttpEndpoint.ValueString()
	} else if exists {
		httpEndpoint = apiBaseUrl
	}

	var clients providerApi
	clients.httpEndpoint = httpEndpoint
	clients.tokens = tokens.Parse(token)

	enableTracing := false
	_, ok := os.LookupEnv("DEBUG")
	if ok {
		enableTracing = true
		resp.Diagnostics.AddWarning("Debug mode enabled", "Debug mode enabled, this will add the Fly-Force-Trace header to all graphql requests")
	}

	fly.SetBaseURL(baseUrl)
	clients.apiClient = *fly.NewClientFromOptions(fly.ClientOptions{
		AccessToken: p.token,
		//Tokens:           nil,
		Name:    "terraform-provider-fly",
		Version: p.version,
		//BaseURL:          "",
		Logger:           logger{ctx},
		EnableDebugTrace: &enableTracing,
		//Transport:        nil,
	})
	clients.gqlClient = clients.apiClient.GenqClient()
	fly.SetErrorLog(true)

	resp.ResourceData = &clients
	resp.DataSourceData = &clients
}

func (p *provider) Resources(ctx context.Context) []func() resource.Resource {
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

func (p *provider) DataSources(ctx context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{
		newAppDataSource,
		newCertDataSource,
		newIpDataSource,
		newOrganizationDataSource,
		NewVolumeDataSource,
	}
}

func (p *provider) Metadata(ctx context.Context, req tfsdkprovider.MetadataRequest, resp *tfsdkprovider.MetadataResponse) {
	resp.TypeName = "fly"
}

func (p *provider) Schema(ctx context.Context, req tfsdkprovider.SchemaRequest, resp *tfsdkprovider.SchemaResponse) {
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
