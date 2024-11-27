package provider

import (
	"context"
	"errors"
	"fmt"
	"github.com/fly-apps/terraform-provider-fly/internal/test"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"testing"
)

func TestAccAppResource_basic(t *testing.T) {
	name := test.GenerateName()

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
resource "fly_app" "test" {
	name = "%s"
	org = "%s"
}
`, name, test.GetTestOrg()),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("fly_app.test", "name", name),
					resource.TestCheckResourceAttr("fly_app.test", "org", test.GetTestOrg()),
					resource.TestCheckResourceAttrSet("fly_app.test", "orgid"),
					resource.TestCheckResourceAttrSet("fly_app.test", "id"),
				),
			},
			{
				Config: fmt.Sprintf(`
resource "fly_app" "test" {
	name = "%s"
	org = "%s"
}
`, name, test.GetTestOrg()),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("fly_app.test", plancheck.ResourceActionNoop),
					},
				},
			},
		},
	})
}

func TestAccAppResource_secrets_basic(t *testing.T) {
	name := test.GenerateName()

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
resource "fly_app" "test" {
	name = "%s"
	org = "%s"
	secrets = {
		TEST = {value = "123"}
	}
}
`, name, test.GetTestOrg()),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("fly_app.test", "name", name),
					resource.TestCheckResourceAttr("fly_app.test", "org", test.GetTestOrg()),
					resource.TestCheckResourceAttrSet("fly_app.test", "orgid"),
					resource.TestCheckResourceAttrSet("fly_app.test", "id"),
					resource.TestCheckResourceAttr("fly_app.test", "secrets.TEST.value", "123"),
					resource.TestCheckResourceAttrSet("fly_app.test", "secrets.TEST.digest"),
					resource.TestCheckResourceAttrSet("fly_app.test", "secrets.TEST.created_at"),
				),
			},
			{
				Config: fmt.Sprintf(`
resource "fly_app" "test" {
	name = "%s"
	org = "%s"
	secrets = {
		TEST = {value = "321"}
	}
}
`, name, test.GetTestOrg()),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("fly_app.test", plancheck.ResourceActionUpdate),
					},
				},
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("fly_app.test", "secrets.TEST.value", "321"),
					resource.TestCheckResourceAttrSet("fly_app.test", "secrets.TEST.digest"),
					resource.TestCheckResourceAttrSet("fly_app.test", "secrets.TEST.created_at"),
				),
			},
		},
	})
}

// Test that we detect and correct when secrets are changed outside terraform
func TestAccAppResource_secrets_drift(t *testing.T) {
	name := test.GenerateName()

	client := test.GetFlyClient()

	var digest1 string
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
resource "fly_app" "test" {
	name = "%s"
	org = "%s"
	secrets = {
		TEST = {value = "321"}
	}
}
`, name, test.GetTestOrg()),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("fly_app.test", "secrets.TEST.value", "321"),
					resource.TestCheckResourceAttrSet("fly_app.test", "secrets.TEST.digest"),
					resource.TestCheckResourceAttrSet("fly_app.test", "secrets.TEST.created_at"),
				),
			},
			// Secret drift detection
			{
				PreConfig: func() {
					s, err := client.FlyGo.GetAppSecrets(context.Background(), name)
					if err != nil {
						t.Fatal(err)
					}
					oldSecret := s[0]
					_, err = client.FlyGo.SetSecrets(context.Background(), name, map[string]string{
						"TEST": "1234",
					})
					if err != nil {
						t.Fatal(err)
					}
					s, err = client.FlyGo.GetAppSecrets(context.Background(), name)
					newSecret := s[0]
					if newSecret.Digest == oldSecret.Digest {
						t.Fatal("fly API SetSecret had no effect")
					}
					digest1 = oldSecret.Digest
				},
				Config: fmt.Sprintf(`
resource "fly_app" "test" {
	name = "%s"
	org = "%s"
	secrets = {
		TEST = {value = "321"}
	}
}
`, name, test.GetTestOrg()),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("fly_app.test", "secrets.TEST.value", "321"),
					resource.TestCheckResourceAttrSet("fly_app.test", "secrets.TEST.digest"),
					resource.TestCheckResourceAttrSet("fly_app.test", "secrets.TEST.created_at"),
					func(state *terraform.State) error {
						s, err := client.FlyGo.GetAppSecrets(context.Background(), name)
						if err != nil {
							return err
						}
						if len(s) != 1 {
							return errors.New(fmt.Sprintf("Unexpected number of secrets %d", len(s)))
						}
						if s[0].Name != "TEST" {
							return errors.New(fmt.Sprintf("Unexpected secret name %v", s[0].Name))
						}
						return nil
					},
					resource.TestCheckResourceAttrWith("fly_app.test", "secrets.TEST.digest", func(value string) error {
						s, err := client.FlyGo.GetAppSecrets(context.Background(), name)
						if err != nil {
							return err
						}
						if s[0].Digest != value {
							return errors.New(fmt.Sprintf("digest in state (%s) differs from digest from fly API (%s)", value, s[0].Digest))
						}
						if value != digest1 {
							return errors.New(fmt.Sprintf("digest in state (%s) differs from digest of same value before drift (%s)", value, digest1))
						}
						return nil
					}),
				),
			},
		},
	})
}

// Verify that we don't touch unmanaged secrets
func TestAccAppResource_secrets_unmanaged(t *testing.T) {
	name := test.GenerateName()

	client := test.GetFlyClient()

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
resource "fly_app" "test" {
	name = "%s"
	org = "%s"
	secrets = {
		TEST = {value = "123"}
	}
}
`, name, test.GetTestOrg()),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("fly_app.test", "secrets.TEST.value", "123"),
				),
			},
			{
				PreConfig: func() {
					_, err := client.FlyGo.SetSecrets(context.Background(), name, map[string]string{
						"UNMANAGED": "1",
					})
					if err != nil {
						t.Fatal(err)
					}
				},
				Config: fmt.Sprintf(`
resource "fly_app" "test" {
	name = "%s"
	org = "%s"
	secrets = {
		TEST = {value = "3"}
	}
}
`, name, test.GetTestOrg()),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("fly_app.test", "secrets.TEST.digest"),
					resource.TestCheckNoResourceAttr("fly_app.test", "secrets.UNMANAGED.digest"),
					func(state *terraform.State) error {
						r, err := client.FlyGo.GetAppSecrets(context.Background(), name)
						if err != nil {
							return err
						}

						if len(r) == 1 && r[0].Name == "TEST" {
							return errors.New("unmanaged secret disappeared")
						} else if len(r) != 2 {
							return errors.New(fmt.Sprintf("unexpected secrets in API response %v", r))
						}
						return nil
					},
				),
			},
			{
				Config: fmt.Sprintf(`
resource "fly_app" "test" {
	name = "%s"
	org = "%s"
	secrets = {}
}
`, name, test.GetTestOrg()),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckNoResourceAttr("fly_app.test", "secrets.TEST"),
					resource.TestCheckNoResourceAttr("fly_app.test", "secrets.UNMANAGED.digest"),
					func(state *terraform.State) error {
						r, err := client.FlyGo.GetAppSecrets(context.Background(), name)
						if err != nil {
							return err
						}

						if len(r) == 0 {
							return errors.New("unmanaged secret disappeared")
						} else if len(r) > 2 || r[0].Name != "UNMANAGED" {
							return errors.New(fmt.Sprintf("unexpected secrets in API %v", r))
						}
						return nil
					},
				),
			},
		},
	})
}

func init() {
	resource.AddTestSweepers(test.FlyAppSweeper.Name, &test.FlyAppSweeper)
}
