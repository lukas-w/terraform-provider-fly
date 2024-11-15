package provider

import (
	"context"
	"errors"
	"fmt"
	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"testing"
)

func TestAccApp_basic(t *testing.T) {
	appName := "testApp"
	resourceName := fmt.Sprintf("fly_app.%s", appName)
	name := acctest.RandStringFromCharSet(10, acctest.CharSetAlphaNum)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
%s
resource "fly_app" "%s" {
	name = "%s"
	org = "%s"
}
`, providerConfig(), appName, name, getTestOrg()),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(resourceName, "name", name),
					resource.TestCheckResourceAttr(resourceName, "org", getTestOrg()),
					resource.TestCheckResourceAttrSet(resourceName, "orgid"),
					resource.TestCheckResourceAttrSet(resourceName, "id"),
				),
			},
			{
				Config: fmt.Sprintf(`
%s
resource "fly_app" "%s" {
	name = "%s"
	org = "%s"
}
`, providerConfig(), appName, name, getTestOrg()),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(resourceName, plancheck.ResourceActionNoop),
					},
				},
			},
		},
	})
}

func TestAccApp_secrets_basic(t *testing.T) {
	appName := "testApp"
	resourceName := fmt.Sprintf("fly_app.%s", appName)
	name := acctest.RandStringFromCharSet(10, acctest.CharSetAlphaNum)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
%s
resource "fly_app" "%s" {
	name = "%s"
	org = "%s"
	secrets = {
		TEST = {value = "123"}
	}
}
`, providerConfig(), appName, name, getTestOrg()),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(resourceName, "name", name),
					resource.TestCheckResourceAttr(resourceName, "org", getTestOrg()),
					resource.TestCheckResourceAttrSet(resourceName, "orgid"),
					resource.TestCheckResourceAttrSet(resourceName, "id"),
					resource.TestCheckResourceAttr(resourceName, "secrets.TEST.value", "123"),
					resource.TestCheckResourceAttrSet(resourceName, "secrets.TEST.digest"),
					resource.TestCheckResourceAttrSet(resourceName, "secrets.TEST.created_at"),
				),
			},
			{
				Config: fmt.Sprintf(`
%s
resource "fly_app" "%s" {
	name = "%s"
	org = "%s"
	secrets = {
		TEST = {value = "321"}
	}
}
`, providerConfig(), appName, name, getTestOrg()),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(resourceName, plancheck.ResourceActionUpdate),
					},
				},
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(resourceName, "secrets.TEST.value", "321"),
					resource.TestCheckResourceAttrSet(resourceName, "secrets.TEST.digest"),
					resource.TestCheckResourceAttrSet(resourceName, "secrets.TEST.created_at"),
				),
			},
		},
	})
}

func TestAccApp_secrets_drift(t *testing.T) {
	appName := "testApp"
	resourceName := fmt.Sprintf("fly_app.%s", appName)
	name := acctest.RandStringFromCharSet(10, acctest.CharSetAlphaNum)

	client := getFlyClient()

	var digest1 string
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
%s
resource "fly_app" "%s" {
	name = "%s"
	org = "%s"
	secrets = {
		TEST = {value = "321"}
	}
}
`, providerConfig(), appName, name, getTestOrg()),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(resourceName, "secrets.TEST.value", "321"),
					resource.TestCheckResourceAttrSet(resourceName, "secrets.TEST.digest"),
					resource.TestCheckResourceAttrSet(resourceName, "secrets.TEST.created_at"),
				),
			},
			// Secret drift detection
			{
				PreConfig: func() {
					s, err := client.GetAppSecrets(context.Background(), name)
					if err != nil {
						t.Fatal(err)
					}
					oldSecret := s[0]
					_, err = client.SetSecrets(context.Background(), name, map[string]string{
						"TEST": "1234",
					})
					if err != nil {
						t.Fatal(err)
					}
					s, err = client.GetAppSecrets(context.Background(), name)
					newSecret := s[0]
					if newSecret.Digest == oldSecret.Digest {
						t.Fatal("fly API SetSecret had no effect")
					}
					digest1 = oldSecret.Digest
				},
				Config: fmt.Sprintf(`
%s
resource "fly_app" "%s" {
	name = "%s"
	org = "%s"
	secrets = {
		TEST = {value = "321"}
	}
}
`, providerConfig(), appName, name, getTestOrg()),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(resourceName, "secrets.TEST.value", "321"),
					resource.TestCheckResourceAttrSet(resourceName, "secrets.TEST.digest"),
					resource.TestCheckResourceAttrSet(resourceName, "secrets.TEST.created_at"),
					func(state *terraform.State) error {
						s, err := client.GetAppSecrets(context.Background(), name)
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
					resource.TestCheckResourceAttrWith(resourceName, "secrets.TEST.digest", func(value string) error {
						s, err := client.GetAppSecrets(context.Background(), name)
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
func TestAccApp_secrets_unmanaged(t *testing.T) {
	appName := "testApp"
	resourceName := fmt.Sprintf("fly_app.%s", appName)
	name := acctest.RandStringFromCharSet(10, acctest.CharSetAlphaNum)

	client := getFlyClient()

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
%s
resource "fly_app" "%s" {
	name = "%s"
	org = "%s"
	secrets = {
		TEST = {value = "123"}
	}
}
`, providerConfig(), appName, name, getTestOrg()),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(resourceName, "secrets.TEST.value", "123"),
				),
			},
			{
				PreConfig: func() {
					_, err := client.SetSecrets(context.Background(), name, map[string]string{
						"UNMANAGED": "1",
					})
					if err != nil {
						t.Fatal(err)
					}
				},
				Config: fmt.Sprintf(`
%s
resource "fly_app" "%s" {
	name = "%s"
	org = "%s"
	secrets = {
		TEST = {value = "3"}
	}
}
`, providerConfig(), appName, name, getTestOrg()),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet(resourceName, "secrets.TEST.digest"),
					resource.TestCheckNoResourceAttr(resourceName, "secrets.UNMANAGED.digest"),
					func(state *terraform.State) error {
						r, err := client.GetAppSecrets(context.Background(), name)
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
%s
resource "fly_app" "%s" {
	name = "%s"
	org = "%s"
	secrets = {}
}
`, providerConfig(), appName, name, getTestOrg()),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckNoResourceAttr(resourceName, "secrets.TEST"),
					resource.TestCheckNoResourceAttr(resourceName, "secrets.UNMANAGED.digest"),
					func(state *terraform.State) error {
						r, err := client.GetAppSecrets(context.Background(), name)
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
