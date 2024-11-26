package provider

import (
	"fmt"
	"github.com/fly-apps/terraform-provider-fly/internal/test"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"os"
	"testing"
)

func TestAccFlyMachineBase(t *testing.T) {
	t.Parallel()
	rName := test.GenerateName()
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		PreCheck:                 func() { testAccPreCheck(t) },
		Steps: []resource.TestStep{
			{
				Config: testFlyMachineResourceConfig(rName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("fly_machine.testMachine", "name", rName),
					resource.TestCheckResourceAttr("fly_machine.testMachine", "app", os.Getenv("FLY_TF_TEST_APP")),
					resource.TestCheckResourceAttr("fly_machine.testMachine", "region", getTestRegion()),
					resource.TestCheckResourceAttr("fly_machine.testMachine", "config.image", "nginx"),
					resource.ComposeTestCheckFunc(
						resource.TestCheckResourceAttrSet("fly_machine.testMachine", "config.guest.cpus"),
						resource.TestCheckResourceAttrSet("fly_machine.testMachine", "config.guest.cpu_kind"),
						resource.TestCheckResourceAttrSet("fly_machine.testMachine", "config.guest.memory_mb"),
					),
					resource.ComposeAggregateTestCheckFunc(
						resource.TestCheckResourceAttr("fly_machine.testMachine", "config.services.0.protocol", "tcp"),
						resource.TestCheckResourceAttr("fly_machine.testMachine", "config.services.0.internal_port", "80"),
					),
					resource.TestCheckResourceAttrSet("fly_machine.testMachine", "private_ip"),
					resource.TestCheckNoResourceAttr("fly_machine.testMachine", "config.restart"),
					resource.TestCheckNoResourceAttr("fly_machine.testMachine", "config.processes"),
					resource.TestCheckNoResourceAttr("fly_machine.testMachine", "config.metrics"),
					resource.TestCheckNoResourceAttr("fly_machine.testMachine", "config.checks"),
				),
			},
			{
				Config: testFlyMachineResourceUpdateConfig(rName),
				Check:  resource.TestCheckResourceAttr("fly_machine.testMachine", "config.env.updatedkey", "updatedValue"),
			},
		},
	})
}

func testFlyMachineResourceConfig(name string) string {
	app := os.Getenv("FLY_TF_TEST_APP")

	return fmt.Sprintf(`
%s

resource "fly_machine" "testMachine" {
	app = "%s"
	region = "%s"
	name = "%s"
	config = {
    	image = "nginx"
		env = {
			updatedkey = "value"
		}
		services = [
		  {
			ports = [
			  {
				port     = 443
				handlers = ["tls", "http"]
			  },
			  {
				port     = 80
				handlers = ["http"]
			  }
			]
			"protocol" : "tcp",
			"internal_port" : 80
		  }
		]
	}
}
`, providerConfig(), app, getTestRegion(), name)
}

func testFlyMachineResourceUpdateConfig(name string) string {
	app := os.Getenv("FLY_TF_TEST_APP")

	return fmt.Sprintf(`
%s

resource "fly_machine" "testMachine" {
	app = "%s"
	region = "%s"
	name = "%s"
	config = {
		image = "nginx"
		env = {
			updatedkey = "updatedValue"
		}
		services = [
		  {
			ports = [
			  {
				port     = 443
				handlers = ["tls", "http"]
			  },
			  {
				port     = 80
				handlers = ["http"]
			  }
			]
			"protocol" : "tcp",
			"internal_port" : 80
		  }
		]
	}
}
`, providerConfig(), app, getTestRegion(), name)
}

func TestAccFlyMachineNoServices(t *testing.T) {
	t.Parallel()
	rName := test.GenerateName()
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		PreCheck:                 func() { testAccPreCheck(t) },
		Steps: []resource.TestStep{
			{
				Config: testFlyMachineResourceNoServicesConfig(rName),
				Check:  resource.TestCheckResourceAttr("fly_machine.testMachine", "name", rName),
			},
		},
	})
}

func testFlyMachineResourceNoServicesConfig(name string) string {
	app := os.Getenv("FLY_TF_TEST_APP")

	return fmt.Sprintf(`
%s

resource "fly_machine" "testMachine" {
	app = "%s"
	region = "%s"
	name = "%s"
	config = {
    	image = "nginx"
		env = {
			updatedkey = "value"
		}
	}
}
`, providerConfig(), app, getTestRegion(), name)
}

func TestAccFlyMachineEmptyServices(t *testing.T) {
	t.Parallel()
	rName := test.GenerateName()
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		PreCheck:                 func() { testAccPreCheck(t) },
		Steps: []resource.TestStep{
			{
				Config: testFlyMachineResourceEmptyServicesConfig(rName),
				Check:  resource.TestCheckResourceAttr("fly_machine.testMachine", "name", rName),
			},
		},
	})
}

func testFlyMachineResourceEmptyServicesConfig(name string) string {
	app := os.Getenv("FLY_TF_TEST_APP")

	return fmt.Sprintf(`
%s

resource "fly_machine" "testMachine" {
	app = "%s"
	region = "%s"
	name = "%s"
	config = {
		image = "nginx"
		env = {
			updatedkey = "value"
		}
		services = []
	}
}
`, providerConfig(), app, getTestRegion(), name)
}

func TestAccFlyMachineInitOptions(t *testing.T) {
	t.Parallel()
	rName := test.GenerateName()
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		PreCheck:                 func() { testAccPreCheck(t) },
		Steps: []resource.TestStep{
			{
				Config: testFlyMachineResourceInitOptionsConfig(rName),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("fly_machine.testMachine", "name", rName),
					resource.TestCheckResourceAttr("fly_machine.testMachine", "config.init.cmd.0", "cmdText"),
					resource.TestCheckResourceAttr("fly_machine.testMachine", "config.init.entrypoint.0", "entrypointText"),
					resource.TestCheckResourceAttr("fly_machine.testMachine", "config.init.exec.0", "execText"),
				),
			},
		},
	})
}

func testFlyMachineResourceInitOptionsConfig(name string) string {
	app := os.Getenv("FLY_TF_TEST_APP")

	return fmt.Sprintf(`
%s

resource "fly_machine" "testMachine" {
	app = "%s"
	region = "%s"
	name = "%s"
	config = {
		image = "nginx"
		env = {
			updatedkey = "value"
		}
		init = {
			cmd = ["cmdText"]
			entrypoint = ["entrypointText"]
			exec = ["execText"]
		}
	}
}
`, providerConfig(), app, getTestRegion(), name)
}

func TestAccFlyMachineModifyImage(t *testing.T) {
	t.Parallel()
	rName := test.GenerateName()
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		PreCheck:                 func() { testAccPreCheck(t) },
		Steps: []resource.TestStep{
			{
				Config: testFlyMachineResourceConfig(rName),
			},
			{
				Config: testFlyMachineResourceChangeImageConfig(rName),
			},
		},
	})
}

func testFlyMachineResourceChangeImageConfig(name string) string {
	app := os.Getenv("FLY_TF_TEST_APP")

	return fmt.Sprintf(`
%s

resource "fly_machine" "testMachine" {
	app = "%s"
	region = "%s"
	name = "%s"
	config = {
		image = "nginx:latest"
		env = {
			updatedkey = "value"
		}
		services = [
		  {
			ports = [
			  {
				port     = 443
				handlers = ["tls", "http"]
			  },
			  {
				port     = 80
				handlers = ["http"]
			  }
			]
			"protocol" : "tcp",
			"internal_port" : 80
		  }
		]
	}
}
`, providerConfig(), app, getTestRegion(), name)
}

func TestAccFlyMachineEmptyName(t *testing.T) {
	t.Parallel()
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		PreCheck:                 func() { testAccPreCheck(t) },
		Steps: []resource.TestStep{
			{
				Config: testFlyMachineResourceEmptyNameConfig(),
			},
			{
				Config: testFlyMachineResourceEmptyNameUpdateConfig(),
			},
		},
	})
}

func testFlyMachineResourceEmptyNameConfig() string {
	app := os.Getenv("FLY_TF_TEST_APP")

	return fmt.Sprintf(`
%s

resource "fly_machine" "testMachine" {
	app = "%s"
	region = "%s"
	config = {
		image = "nginx"
		env = {
			updatedkey = "value"
		}
		services = [
		  {
			ports = [
			  {
				port     = 443
				handlers = ["tls", "http"]
			  },
			  {
				port     = 80
				handlers = ["http"]
			  }
			]
			"protocol" : "tcp",
			"internal_port" : 80
		  }
		]
	}
}
`, providerConfig(), app, getTestRegion())
}

func testFlyMachineResourceEmptyNameUpdateConfig() string {
	app := os.Getenv("FLY_TF_TEST_APP")

	return fmt.Sprintf(`
%s

resource "fly_machine" "testMachine" {
	app = "%s"
	region = "%s"
	config = {
		image = "nginx:latest"
		env = {
			updatedkey = "value"
		}
		services = [
		  {
			ports = [
			  {
				port     = 443
				handlers = ["tls", "http"]
			  },
			  {
				port     = 80
				handlers = ["http"]
			  }
			]
			"protocol" : "tcp",
			"internal_port" : 80
		  }
		]
	}
}
`, providerConfig(), app, getTestRegion())
}

func TestAccFlyMachineGuest(t *testing.T) {
	t.Parallel()
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		PreCheck:                 func() { testAccPreCheck(t) },
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
%s

resource "fly_machine" "testMachine" {
	app = "%s"
	region = "%s"
	config = {
		image = "nginx:latest"
		guest = {
			memory_mb = 256
			cpus = 1
			cpu_kind = "shared"
		}
	}
}
`, providerConfig(), os.Getenv("FLY_TF_TEST_APP"), getTestRegion()),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("fly_machine.testMachine", "config.guest.memory_mb", "256"),
					resource.TestCheckResourceAttr("fly_machine.testMachine", "config.guest.cpus", "1"),
					resource.TestCheckResourceAttr("fly_machine.testMachine", "config.guest.cpu_kind", "shared"),
					resource.TestCheckNoResourceAttr("fly_machine.testMachine", "config.guest.kernel_args"),
				),
			},
		},
	})
}

func TestAccFlyMachineChecks(t *testing.T) {
	t.Parallel()
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		PreCheck:                 func() { testAccPreCheck(t) },
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
%s

resource "fly_machine" "testMachine" {
	app = "%s"
	region = "%s"
	config = {
		image = "nginx:latest"
		
		checks = {
			pg = {
				type      = "http"
				port      = 5500
				http_path = "/check"
				interval  = "1m"
				timeout   = "5s"
			}
		}
	}
}
`, providerConfig(), os.Getenv("FLY_TF_TEST_APP"), getTestRegion()),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("fly_machine.testMachine", "config.checks.pg.type", "http"),
					resource.TestCheckResourceAttr("fly_machine.testMachine", "config.checks.pg.port", "5500"),
				),
			},
		},
	})
}
