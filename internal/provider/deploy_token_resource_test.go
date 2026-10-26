package provider

import (
	"fmt"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/lukas-w/terraform-provider-fly/internal/test"
)

func TestAccDeployTokenResource_basic(t *testing.T) {
	if os.Getenv("FLY_TF_TEST_TOKENS") != "1" {
		t.Skip("Skipping deploy token test because FLY_TF_TEST_TOKENS is not set to 1")
	}
	appName := test.GetTestAppName()
	tokenName := test.GenerateName()

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
resource "fly_deploy_token" "test" {
	name = "%s"
	app = "%s"
}
`, tokenName, appName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("fly_deploy_token.test", "name", tokenName),
					resource.TestCheckResourceAttrSet("fly_deploy_token.test", "id"),
					resource.TestCheckResourceAttrSet("fly_deploy_token.test", "token"),
				),
			},
		},
	})
}
