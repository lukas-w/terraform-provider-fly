package test

import (
	"context"
	"github.com/lukas-w/terraform-provider-fly/graphql"
	"github.com/lukas-w/terraform-provider-fly/internal/flyapi"
	"github.com/superfly/fly-go"
	"log"
	"os"
)

var clients *flyapi.Clients
var testApp *fly.App

func GetFlyClient() *flyapi.Clients {
	if clients == nil {
		clients = flyapi.New(
			context.Background(),
			flyapi.Options{
				AccessToken: os.Getenv("FLY_API_TOKEN"),
				Version:     "test",
			},
		)
	}
	return clients
}

func GetTestOrg() string {
	org, ok := os.LookupEnv("FLY_TF_TEST_ORG")
	if !ok {
		log.Fatal("FLY_TF_TEST_ORG not set")
	}
	return org
}

func GetTestOrgID() string {
	slug := GetTestOrg()
	org, err := graphql.Organization(context.Background(), GetFlyClient().Gql, "", "", slug)
	if err != nil {
		log.Fatalf("Error getting org: %v", err)
	}
	return org.Organization.Id
}

func GetTestRegion() string {
	region, ok := os.LookupEnv("FLY_TF_TEST_REGION")
	if ok {
		return region
	} else {
		return "ewr"
	}
}

func GetTestAppName() string {
	if testApp == nil {
		log.Fatal("Test app not created")
	}
	return testApp.Name
}

func CreateTestApp() (string, error) {
	name := GenerateName()
	app, err := GetFlyClient().FlyGo.CreateApp(
		context.Background(), fly.CreateAppInput{
			Name:           name,
			OrganizationID: GetTestOrgID(),
		},
	)
	if err != nil {
		return "", err
	}
	testApp = app
	return testApp.Name, nil
}

func DeleteTestApp() error {
	if testApp == nil {
		log.Print("Test app not created")
		return nil
	}
	err := GetFlyClient().FlyGo.DeleteApp(context.Background(), testApp.Name)
	if err != nil {
		return err
	}
	return nil
}
