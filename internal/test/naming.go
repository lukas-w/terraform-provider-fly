package test

import (
	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"strings"
)

const Prefix = "tf-acc-test-"

func GenerateName() string {
	return Prefix + acctest.RandString(5)
}

func HasPrefix(name string) bool {
	return strings.HasPrefix(name, Prefix)
}
