package provider

import (
	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/lukas-w/terraform-provider-fly/internal/test"
	"log"
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
}

type MWithTestApp struct {
	m *testing.M
}

func (a MWithTestApp) Run() int {
	// Create one test app for all tests that need one
	appName, err := test.CreateTestApp()
	if err != nil {
		log.Fatalf("Error creating test app: %v", err)
	} else {
		log.Printf("Created test app %s", appName)
	}
	code := a.m.Run()

	err = test.DeleteTestApp()
	if err != nil {
		log.Fatalf("Error deleting test app: %v", err)
	}

	return code
}

func TestMain(m *testing.M) {
	// resource.TestMain doesn't call m.Run() when sweeping. Putting out test app creation & deletion
	// within m.Run() therefore ensures that it's only run when actually running tests.
	resource.TestMain(
		MWithTestApp{
			m: m,
		},
	)
}
