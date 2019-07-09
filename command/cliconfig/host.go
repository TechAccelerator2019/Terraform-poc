package cliconfig

import (
	"github.com/hashicorp/terraform/svchost"
)

// Host represents a "host" configuration block within the CLI configuration,
// which can be used to override the default service host discovery behavior
// for a particular hostname.
type Host struct {
	Host     svchost.Hostname
	Services map[string]interface{}
}
