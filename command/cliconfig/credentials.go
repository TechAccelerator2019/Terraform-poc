package cliconfig

import (
	"github.com/hashicorp/terraform/svchost"
)

// Credentials represents a "credentials" block in the CLI configuration.
type Credentials struct {
	Host svchost.Hostname
	Raw  map[string]interface{}
}

// CredentialsHelper represents a "credentials_helper" block in the CLI
// configuration.
type CredentialsHelper struct {
	Type string
	Args []string `hcl:"args"`
}
