package flyapi

import (
	"context"
	"fmt"
	"github.com/Khan/genqlient/graphql"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	flyGo "github.com/superfly/fly-go"
	"github.com/superfly/fly-go/flaps"
	flyGoTokens "github.com/superfly/fly-go/tokens"
)

const DefaultBaseUrl = "https://api.fly.io"

type FlyTfLogger struct {
	context.Context
}

func (l FlyTfLogger) Debug(v ...interface{}) {
	tflog.Debug(l.Context, fmt.Sprint(v...))
}

func (l FlyTfLogger) Debugf(format string, v ...interface{}) {
	tflog.Debug(l.Context, fmt.Sprintf(format, v...))
}

type GqlClient graphql.Client

type Clients struct {
	FlyGo        flyGo.Client
	Gql          GqlClient
	flapsClients map[string]*flaps.Client
	tokens       *flyGoTokens.Tokens
}

type Options struct {
	BaseURL          string
	AccessToken      string
	Version          string
	EnableDebugTrace bool
}

func New(ctx context.Context, opts Options) (c *Clients) {
	if opts.BaseURL == "" {
		opts.BaseURL = DefaultBaseUrl
	}
	c = &Clients{}
	c.tokens = flyGoTokens.Parse(opts.AccessToken)
	c.FlyGo = *flyGo.NewClientFromOptions(
		flyGo.ClientOptions{
			BaseURL:          opts.BaseURL,
			Name:             "terraform-provider-fly",
			Logger:           FlyTfLogger{Context: ctx},
			Tokens:           c.tokens,
			EnableDebugTrace: &opts.EnableDebugTrace,
		},
	)
	c.Gql = c.FlyGo.GenqClient()
	return
}

func (c *Clients) GetFlapsClient(ctx context.Context, app string) (*flaps.Client, error) {
	if c.flapsClients == nil {
		c.flapsClients = make(map[string]*flaps.Client)
	}

	if client, ok := c.flapsClients[app]; ok {
		return client, nil
	}

	appCompact, err := c.FlyGo.GetAppCompact(ctx, app)
	if err != nil {
		return nil, err
	}
	client, err := flaps.NewWithOptions(
		ctx, flaps.NewClientOpts{
			AppName:    app,
			AppCompact: appCompact,
			Tokens:     c.tokens,
		},
	)
	if err != nil {
		return nil, err
	}

	c.flapsClients[app] = client
	return client, nil
}
