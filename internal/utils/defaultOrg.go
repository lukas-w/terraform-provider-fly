package utils

import (
	"context"
	"errors"
	"github.com/superfly/fly-go"
)

func GetDefaultOrg(ctx context.Context, client fly.Client) (*fly.Organization, error) {
	orgs, err := client.GetOrganizations(ctx)
	if err != nil {
		return nil, err
	}
	if len(orgs) > 1 {
		return nil, errors.New("organization is ambiguous. Your account has more than one organization, you must specify which to use")
	}
	if len(orgs) < 1 {
		return nil, errors.New("no organizations to choose from. This error should not be reachable, if you find it please file an issue")
	}
	return &orgs[0], nil
}
