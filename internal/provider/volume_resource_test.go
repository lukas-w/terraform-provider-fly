package provider

import (
	"fmt"
	"github.com/fly-apps/terraform-provider-fly/internal/test"
	"github.com/hashicorp/terraform-plugin-testing/compare"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
	"testing"
)

func TestAccVolumeResource(t *testing.T) {
	// name := test.GenerateName()
	name := "test"
	app := test.GetTestAppName()
	r := "fly_volume.test"

	compareIdSame := statecheck.CompareValue(compare.ValuesSame())
	compareIdDiffer := statecheck.CompareValue(compare.ValuesDiffer())

	resource.Test(
		t, resource.TestCase{
			ProtoV6ProviderFactories: testAccProtoV6ProviderFactories, // CheckDestroy:             testAccCheckVolumeDestroy,
			Steps: []resource.TestStep{
				{
					Config: fmt.Sprintf(
						`
resource "fly_volume" "test" {
	app = "%s"  
	name = "%s"
	size = 1
	region = "fra" 
}
`, app, name,
					),
					ConfigStateChecks: []statecheck.StateCheck{
						statecheck.ExpectKnownValue(r, tfjsonpath.New("id"), knownvalue.NotNull()),
						compareIdSame.AddStateValue(r, tfjsonpath.New("id")),
						statecheck.ExpectKnownValue(
							r, tfjsonpath.New("app"), knownvalue.StringExact(app),
						),
						statecheck.ExpectKnownValue(
							r, tfjsonpath.New("name"), knownvalue.StringExact("test"),
						),
						statecheck.ExpectKnownValue(
							r, tfjsonpath.New("size"), knownvalue.Int64Exact(1),
						),
						statecheck.ExpectKnownValue(
							r, tfjsonpath.New("region"), knownvalue.StringExact("fra"),
						),
						statecheck.ExpectKnownValue(
							r, tfjsonpath.New("encrypted"), knownvalue.Bool(true),
						),
					},
				},
				{
					Config: fmt.Sprintf(
						`
resource "fly_volume" "test" {
	app = "%s"  
	name = "%s"
	size = 2
	region = "fra" 
}
`, app, name,
					),
					ConfigStateChecks: []statecheck.StateCheck{
						compareIdSame.AddStateValue(r, tfjsonpath.New("id")),
						compareIdDiffer.AddStateValue(r, tfjsonpath.New("id")),
						statecheck.ExpectKnownValue(
							r, tfjsonpath.New("size"), knownvalue.Int64Exact(2),
						),
					},
				},
				{
					Config: fmt.Sprintf(
						`
resource "fly_volume" "test" {
	app = "%s"  
	name = "%s"
	size = 1
	region = "fra" 
}
`, app, name,
					),
					ConfigStateChecks: []statecheck.StateCheck{
						// Volume should have been recreated and thus have a new ID because
						// shrinking is not supported
						compareIdDiffer.AddStateValue(r, tfjsonpath.New("id")),
						statecheck.ExpectKnownValue(
							r, tfjsonpath.New("size"), knownvalue.Int64Exact(1),
						),
					},
				},
			},
		},
	)
}
