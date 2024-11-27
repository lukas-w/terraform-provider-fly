package provider

import (
	"fmt"
	"github.com/fly-apps/terraform-provider-fly/internal/test"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"testing"
)

func TestAccIpResource_basic(t *testing.T) {
	appName := test.GetTestAppName()

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
resource "fly_ip" "ip" {
	app = "%s"
	type = "v6"
	private = true
}
`, appName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("fly_ip.ip", "address"),
					resource.TestCheckResourceAttr("fly_ip.ip", "type", "v6"),
					resource.TestCheckResourceAttr("fly_ip.ip", "private", "true"),
				),
			},
		},
	})
}
