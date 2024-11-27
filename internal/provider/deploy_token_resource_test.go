package provider

import (
	"fmt"
	"github.com/fly-apps/terraform-provider-fly/internal/test"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"testing"
)

func TestAccDeployTokenResource_basic(t *testing.T) {
	appName := test.GetTestAppName()
	tokenName := test.GenerateName()

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
%s
resource "fly_deploy_token" "test" {
	name = "%s"
	app = "%s"
}
`, providerConfig(), tokenName, appName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("fly_deploy_token.test", "name", tokenName),
					resource.TestCheckResourceAttrSet("fly_deploy_token.test", "id"),
					resource.TestCheckResourceAttrSet("fly_deploy_token.test", "token"),
				),
			},
		},
	})
}
