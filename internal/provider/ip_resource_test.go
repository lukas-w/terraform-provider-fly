package provider

import (
	"fmt"
	"github.com/fly-apps/terraform-provider-fly/internal/test"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"testing"
)

func TestAccIp_basic(t *testing.T) {
	appName := test.GenerateName()

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
%s
resource "fly_app" "app" {
	name = "%s"
	org = "%s"
}
resource "fly_ip" "ip" {
	app = fly_app.app.id
	type = "v6"
	private = true
}
`, providerConfig(), appName, getTestOrg()),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("fly_app.app", "id"),
					resource.TestCheckResourceAttrSet("fly_ip.ip", "address"),
					resource.TestCheckResourceAttr("fly_ip.ip", "type", "v6"),
					resource.TestCheckResourceAttr("fly_ip.ip", "private", "true"),
				),
			},
		},
	})
}
