package provider

import (
	"fmt"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/superfly/fly-go"
	"os"
	"testing"
)

func TestAccDeployTokenResource_basic(t *testing.T) {
	appName := os.Getenv("FLY_TF_TEST_APP")
	//tokenName := acctest.RandStringFromCharSet(10, acctest.CharSetAlphaNum)

	fly.SetBaseURL(baseUrl)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
%s
resource "fly_deploy_token" "test" {
	name = "a deploy token"
	app = "%s"
}
`, providerConfig(), appName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("fly_deploy_token.test", "name", "a deploy token"),
					resource.TestCheckResourceAttrSet("fly_deploy_token.test", "id"),
					resource.TestCheckResourceAttrSet("fly_deploy_token.test", "token"),
				),
			},
		},
	})
}
