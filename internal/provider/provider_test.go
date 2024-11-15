package provider

import (
	"context"
	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/superfly/fly-go"
	"os"
	"testing"
)

var testAccProtoV6ProviderFactories = map[string]func() (tfprotov6.ProviderServer, error){
	"fly": providerserver.NewProtocol6WithError(New("test")()),
}

func testAccPreCheck(t *testing.T) {
	_, hasKey := os.LookupEnv("FLY_API_TOKEN")
	if !hasKey {
		t.Fatalf("Need api key in FLY_API_TOKEN")
	}
	_, hasApp := os.LookupEnv("FLY_TF_TEST_APP")
	if !hasApp {
		t.Fatalf("Need app in FLY_TF_TEST_APP")
	}
}

func getFlyClient() fly.Client {
	fly.SetBaseURL(baseUrl)
	return *fly.NewClient(os.Getenv("FLY_API_TOKEN"), "terraform-provider-fly", "test", logger{context.Background()})
}

func getTestOrg() string {
	org, ok := os.LookupEnv("FLY_TF_TEST_ORG")
	if ok {
		return org
	} else {
		return "fly-terraform-ci"
	}
}

func getTestRegion() string {
	region, ok := os.LookupEnv("FLY_TF_TEST_REGION")
	if ok {
		return region
	} else {
		return "ewr"
	}
}

func providerConfig() string {
	return `
provider "fly" {
}
`
}
