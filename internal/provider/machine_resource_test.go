package provider

import (
	"fmt"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/lukas-w/terraform-provider-fly/internal/test"
	"testing"
)

func TestAccMachineResource_Base(t *testing.T) {
	t.Parallel()
	rName := test.GenerateName()
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		PreCheck:                 func() { testAccPreCheck(t) },
		Steps: []resource.TestStep{
			{
				Config: testAccMachineResourceConfig(rName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("fly_machine.testMachine", "name", rName),
					resource.TestCheckResourceAttr("fly_machine.testMachine", "app", test.GetTestAppName()),
					resource.TestCheckResourceAttr("fly_machine.testMachine", "region", test.GetTestRegion()),
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
				Config: testAccMachineResourceUpdateConfig(rName),
				Check:  resource.TestCheckResourceAttr("fly_machine.testMachine", "config.env.updatedkey", "updatedValue"),
			},
		},
	})
}

func testAccMachineResourceConfig(machineName string) string {
	return fmt.Sprintf(`
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
`, test.GetTestAppName(), test.GetTestRegion(), machineName)
}

func testAccMachineResourceUpdateConfig(machineName string) string {
	return fmt.Sprintf(`
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
`, test.GetTestAppName(), test.GetTestRegion(), machineName)
}

func TestAccMachineResource_NoServices(t *testing.T) {
	t.Parallel()
	rName := test.GenerateName()
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		PreCheck:                 func() { testAccPreCheck(t) },
		Steps: []resource.TestStep{
			{
				Config: testAccMachineResourceNoServicesConfig(rName),
				Check:  resource.TestCheckResourceAttr("fly_machine.testMachine", "name", rName),
			},
		},
	})
}

func testAccMachineResourceNoServicesConfig(machineName string) string {
	app := test.GetTestAppName()

	return fmt.Sprintf(`
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
`, app, test.GetTestRegion(), machineName)
}

func TestAccMachineResource_EmptyServices(t *testing.T) {
	t.Parallel()
	rName := test.GenerateName()
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		PreCheck:                 func() { testAccPreCheck(t) },
		Steps: []resource.TestStep{
			{
				Config: testAccMachineResourceEmptyServicesConfig(rName),
				Check:  resource.TestCheckResourceAttr("fly_machine.testMachine", "name", rName),
			},
		},
	})
}

func testAccMachineResourceEmptyServicesConfig(machineName string) string {
	return fmt.Sprintf(`
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
`, test.GetTestAppName(), test.GetTestRegion(), machineName)
}

func TestAccMachineResource_InitOptions(t *testing.T) {
	t.Parallel()
	rName := test.GenerateName()
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		PreCheck:                 func() { testAccPreCheck(t) },
		Steps: []resource.TestStep{
			{
				Config: testAccMachineResourceInitOptionsConfig(rName),
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

func testAccMachineResourceInitOptionsConfig(machineName string) string {
	return fmt.Sprintf(`
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
`, test.GetTestAppName(), test.GetTestRegion(), machineName)
}

func TestAccMachineResource_ModifyImage(t *testing.T) {
	t.Parallel()
	rName := test.GenerateName()
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		PreCheck:                 func() { testAccPreCheck(t) },
		Steps: []resource.TestStep{
			{
				Config: testAccMachineResourceConfig(rName),
			},
			{
				Config: testAccMachineResourceChangeImageConfig(rName),
			},
		},
	})
}

func testAccMachineResourceChangeImageConfig(machineName string) string {
	return fmt.Sprintf(`
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
`, test.GetTestAppName(), test.GetTestRegion(), machineName)
}

func TestAccMachineResource_EmptyName(t *testing.T) {
	t.Parallel()
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		PreCheck:                 func() { testAccPreCheck(t) },
		Steps: []resource.TestStep{
			{
				Config: testAccMachineResourceEmptyNameConfig(),
			},
			{
				Config: testAccMachineResourceEmptyNameUpdateConfig(),
			},
		},
	})
}

func testAccMachineResourceEmptyNameConfig() string {
	return fmt.Sprintf(`
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
`, test.GetTestAppName(), test.GetTestRegion())
}

func testAccMachineResourceEmptyNameUpdateConfig() string {
	return fmt.Sprintf(`
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
`, test.GetTestAppName(), test.GetTestRegion())
}

func TestAccMachineResource_Guest(t *testing.T) {
	t.Parallel()
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		PreCheck:                 func() { testAccPreCheck(t) },
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
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
`, test.GetTestAppName(), test.GetTestRegion()),
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

func TestAccMachineResource_Checks(t *testing.T) {
	t.Parallel()
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		PreCheck:                 func() { testAccPreCheck(t) },
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
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
`, test.GetTestAppName(), test.GetTestRegion()),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("fly_machine.testMachine", "config.checks.pg.type", "http"),
					resource.TestCheckResourceAttr("fly_machine.testMachine", "config.checks.pg.port", "5500"),
				),
			},
		},
	})
}

func TestAccMachineResource_Mounts(t *testing.T) {
	t.Parallel()
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		PreCheck:                 func() { testAccPreCheck(t) },
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
resource "fly_volume" "test" {
	app = "%[1]s"
	size = 3
	name = "test"
	region = "%[2]s"
}
resource "fly_machine" "testMachine" {
	app = "%[1]s"
	region = "%[2]s"
	config = {
		image = "nginx:latest"
		mounts = [
			{
				volume = fly_volume.test.id
				path = "/mnt"
			}
		]
	}
}
`, test.GetTestAppName(), test.GetTestRegion()),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("fly_machine.testMachine", "config.mounts.0.volume"),
					resource.TestCheckResourceAttr("fly_machine.testMachine", "config.mounts.0.path", "/mnt"),
					resource.TestCheckResourceAttr("fly_machine.testMachine", "config.mounts.0.encrypted", "true"),
					resource.TestCheckResourceAttr("fly_machine.testMachine", "config.mounts.0.size_gb", "3"),
				),
			},
		},
	})
}
