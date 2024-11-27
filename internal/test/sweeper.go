package test

import (
	"context"
	"errors"
	"fmt"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"log"
)

var FlyAppSweeper = resource.Sweeper{
	Name: "app",
	F: func(_ string) error {
		ctx := context.Background()
		orgID := GetTestOrgID()
		apps, err := GetFlyClient().FlyGo.GetAppsForOrganization(ctx, orgID)
		log.Printf("Found %d apps", len(apps))
		if err != nil {
			return err
		}
		var errs []error
		for _, app := range apps {
			if err := EnsurePrefix(app.Name, "app"); err != nil {
				return err
			}
			err = GetFlyClient().FlyGo.DeleteApp(context.Background(), app.Name)
			if err != nil {
				log.Printf("Failed deleting app %s: %v", app.Name, err)
				errs = append(errs, err)
			} else {
				log.Printf("Deleted app %s", app.Name)
			}
		}
		return errors.Join(errs...)
	},
}

func EnsurePrefix(name string, resourceType string) error {
	if HasPrefix(name) {
		return nil
	}

	// If there are resources not created by us, user may have run this in a production
	// organization. Stop to prevent destroying their stuff.
	return errors.New(
		fmt.Sprintf(
			"Encountered %s named %s which is lacking test prefix %s in test organization %s. "+
				"Did you run this in a dedicated test org? Aborting to prevent collateral damage.",
			resourceType,
			Prefix,
			GetTestOrg(),
			name,
		),
	)
}
