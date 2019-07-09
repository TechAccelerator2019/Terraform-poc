// Package cliconfig contains the decoder functionality for the CLI
// Configuration.
//
// The CLI configuration is not the same thing as the Terraform configuration.
// Terraform configuration is .tf files in the configuration directory, while
// CLI configuration is .tfrc files in the CLI configuration directory (a
// subdirectory of the user's home directory) along with the .terraformrc file.
package cliconfig

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/hashicorp/terraform/tfdiags"
)

// Config represents the final result of loading all of the CLI configuration
// files.
type Config struct {
	Providers    map[string]*LegacyPluginOverride
	Provisioners map[string]*LegacyPluginOverride

	DisableCheckpoint          bool
	DisableCheckpointSignature bool

	// If set, enables local caching of plugins in this directory to
	// avoid repeatedly re-downloading over the Internet.
	PluginCacheDir string

	Hosts map[string]*Host

	Credentials       map[string]*Credentials
	CredentialsHelper *CredentialsHelper
}

// LoadConfig reads the given files, directories, and environment and assembles
// them together into a single configuration object, or returns errors
// if that isn't possible.
func LoadConfig(configFiles []string, configDirs []string, environ []string) (*Config, tfdiags.Diagnostics) {
	var allFiles []*configFile
	var diags tfdiags.Diagnostics

	for _, fn := range configFiles {
		if _, err := os.Stat(fn); err != nil {
			continue
		}

		f, moreDiags := loadConfigFile(fn, environ)
		diags = diags.Append(moreDiags)
		if !moreDiags.HasErrors() {
			allFiles = append(allFiles, f)
		}
	}

	for _, dn := range configDirs {
		entries, err := ioutil.ReadDir(dn)
		if err != nil {
			continue // Ignore paths that don't behave as directories
		}

		for _, entry := range entries {
			name := entry.Name()
			if configFileFormat(name) == invalidFormat {
				continue
			}

			fn := filepath.Join(dn, name)
			f, moreDiags := loadConfigFile(fn, environ)
			diags = diags.Append(moreDiags)
			if !moreDiags.HasErrors() {
				allFiles = append(allFiles, f)
			}
		}
	}

	allFiles = append(allFiles, environConfig(environ))

	cfg, moreDiags := mergeFiles(allFiles)
	diags = diags.Append(moreDiags)
	return cfg, diags
}

func mergeFiles(files []*configFile) (*Config, tfdiags.Diagnostics) {
	result := &Config{
		Providers:    map[string]*LegacyPluginOverride{},
		Provisioners: map[string]*LegacyPluginOverride{},
		Hosts:        map[string]*Host{},
		Credentials:  map[string]*Credentials{},
	}
	var diags tfdiags.Diagnostics

	credHelperDeclFile := ""

	for _, f := range files {
		for _, provider := range f.Providers {
			result.Providers[provider.Name] = provider
		}
		for _, provisioner := range f.Provisioners {
			result.Provisioners[provisioner.Name] = provisioner
		}
		if f.DisableCheckpoint {
			result.DisableCheckpoint = true
		}
		if f.DisableCheckpointSignature {
			result.DisableCheckpointSignature = true
		}
		if f.PluginCacheDir != "" {
			result.PluginCacheDir = f.PluginCacheDir
		}
		for _, host := range f.Hosts {
			result.Hosts[host.Host.String()] = host
		}
		for _, credentials := range f.Credentials {
			result.Credentials[credentials.Host.String()] = credentials
		}
		if len(f.CredentialsHelpers) != 0 {
			if result.CredentialsHelper != nil {
				diags = diags.Append(tfdiags.Sourceless(
					tfdiags.Error,
					"Multiple credentials_helper blocks",
					fmt.Sprintf("Terraform found credentials_helper blocks in both %s and %s. Only one credentials helper is allowed.", credHelperDeclFile, f.Filename),
				))
			} else {
				result.CredentialsHelper = f.CredentialsHelpers[0]
				credHelperDeclFile = f.Filename
			}
		}
		if len(f.CredentialsHelpers) > 1 {
			diags = diags.Append(tfdiags.Sourceless(
				tfdiags.Error,
				"Multiple credentials_helper blocks",
				fmt.Sprintf("There are multiple credentials_helper blocks in %s. Only one credentials helper is allowed.", f.Filename),
			))
		}
	}

	return result, diags
}
