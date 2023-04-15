package provider

import (
	"fmt"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/acctest"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/superfly/flyctl/api"
	"testing"
)

func TestAccDeployTokenResource_basic(t *testing.T) {
	appName := acctest.RandStringFromCharSet(10, acctest.CharSetAlphaNum)
	//tokenName := acctest.RandStringFromCharSet(10, acctest.CharSetAlphaNum)

	api.SetBaseURL(baseUrl)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
%s
resource "fly_app" "test" {
	name = "%s"
	org = "%s"
}
resource "fly_deploy_token" "test" {
	name = "a deploy token"
	app = fly_app.test.id
}
`, providerConfig(), appName, getTestOrg()),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("fly_deploy_token.test", "name", "a deploy token"),
					resource.TestCheckResourceAttrSet("fly_deploy_token.test", "id"),
					resource.TestCheckResourceAttrSet("fly_deploy_token.test", "token"),
				),
			},
		},
	})
}
