package cliconfig

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	legacyhcl "github.com/hashicorp/hcl"
	legacyhclparser "github.com/hashicorp/hcl/hcl/parser"
	legacyhcltoken "github.com/hashicorp/hcl/hcl/token"
	"github.com/hashicorp/hcl2/gohcl"
	"github.com/hashicorp/hcl2/hcl"
	"github.com/hashicorp/hcl2/hclparse"

	"github.com/hashicorp/terraform/svchost"
	"github.com/hashicorp/terraform/tfdiags"
)

// configFile represents a single configuration file.
//
// Multiple configuration files are merged together to produce a combined
// Config.
type configFile struct {
	Filename string

	Providers    []*LegacyPluginOverride
	Provisioners []*LegacyPluginOverride

	DisableCheckpoint          bool
	DisableCheckpointSignature bool

	PluginCacheDir string

	Hosts []*Host

	Credentials        []*Credentials
	CredentialsHelpers []*CredentialsHelper
}

func loadConfigFile(fn string, environ []string) (*configFile, tfdiags.Diagnostics) {
	// Because we switched to the new HCL implementation in a minor release,
	// we need to be able to fall back to legacy HCL parsing in the event of
	// a syntax error to avoid any breaking changes.
	//
	// Therefore we'll try first parsing with the new parser and fall back
	// to the old if there are problems. If both fail then we'll report the
	// errors from the new parser (they are likely to be higher quality) but
	// if the legacy parser succeeds then we'll prefer to use its result,
	// though we'll still generate a warning in that case to prompt users to
	// fix syntax so that we can switch over to the new parser only in a
	// future major release.

	f, diags := loadConfigFileHCL(fn, environ)
	if diags.HasErrors() {
		legacyF, legacyDiags := loadConfigFileLegacyHCL(fn, environ)
		if legacyDiags.HasErrors() {
			return nil, diags // New parser's diagnostics are generally better
		}
		// We'll also convert all of the errors from the new parser into
		// warnings so the user can see what they need to fix before the
		// backward-compatible parser is removed.
		for _, diag := range diags {
			sev := diag.Severity()
			desc := diag.Description()
			rngs := diag.Source()

			// Converting in this way is a bit lossy (we lose expression-related
			// information, for example) but should be good enough to draw
			// attention to the errors as a stop-gap until we start returning
			// these as hard errors.
			newDiag := &hcl.Diagnostic{
				Severity: hcl.DiagWarning,
				Summary:  desc.Summary,
				Detail:   desc.Detail,
			}

			if sev == tfdiags.Error {
				const suffix = "This message will become a fatal error in a future version of Terraform."
				if newDiag.Detail != "" {
					newDiag.Detail = newDiag.Detail + "\n\n" + suffix
				} else {
					newDiag.Detail = suffix
				}
			}
			if rngs.Subject != nil {
				newDiag.Subject = rngs.Subject.ToHCL().Ptr()
			}
			if rngs.Context != nil {
				newDiag.Context = rngs.Context.ToHCL().Ptr()
			}
			diags = diags.Append(newDiag)
		}
		return legacyF, legacyDiags
	}
	return f, diags
}

func loadConfigFileHCL(fn string, environ []string) (*configFile, tfdiags.Diagnostics) {
	var diags tfdiags.Diagnostics
	result := &configFile{
		Filename: fn,
	}

	p := hclparse.NewParser()

	var hclF *hcl.File
	var hclDiags hcl.Diagnostics
	switch configFileFormat(fn) {
	case hclFormat:
		hclF, hclDiags = p.ParseHCLFile(fn)
	case jsonFormat:
		hclF, hclDiags = p.ParseJSONFile(fn)
	default:
		diags = diags.Append(tfdiags.Sourceless(
			tfdiags.Error,
			"Unsupported CLI config format",
			fmt.Sprintf("Cannot load CLI configuration from %s. CLI configuration files must either be in HCL native syntax (just .tfrc extension) or HCL JSON syntax (.tfrc.json extension).", fn),
		))
	}
	diags = diags.Append(hclDiags)
	if hclDiags.HasErrors() {
		return nil, diags
	}

	// We use PartialContent here so that we'll ignore any new constructs added
	// in future versions of Terraform, because the CLI config is a global
	// artifact shared among all installed Terraform versions.
	content, _, hclDiags := hclF.Body.PartialContent(&hcl.BodySchema{
		Attributes: []hcl.AttributeSchema{
			{Name: "disable_checkpoint"},
			{Name: "disable_checkpoint_signature"},
			{Name: "plugin_cache_dir"},
		},
		Blocks: []hcl.BlockHeaderSchema{
			{Type: "credentials", LabelNames: []string{"hostname"}},
			{Type: "credentials_helper", LabelNames: []string{"type"}},
			{Type: "host", LabelNames: []string{"hostname"}},
			{Type: "providers"},
			{Type: "provisioners"},
		},
	})

	ctx := &hcl.EvalContext{
		Variables: environForHCL(environ),
	}

	if attr, exists := content.Attributes["disable_checkpoint"]; exists {
		moreDiags := gohcl.DecodeExpression(attr.Expr, ctx, &result.DisableCheckpoint)
		diags = diags.Append(moreDiags)
	}

	if attr, exists := content.Attributes["disable_checkpoint_signature"]; exists {
		moreDiags := gohcl.DecodeExpression(attr.Expr, ctx, &result.DisableCheckpointSignature)
		diags = diags.Append(moreDiags)
	}

	if attr, exists := content.Attributes["plugin_cache_dir"]; exists {
		moreDiags := gohcl.DecodeExpression(attr.Expr, ctx, &result.PluginCacheDir)
		diags = diags.Append(moreDiags)
	}

	for _, block := range content.Blocks {
		switch block.Type {
		case "credentials":
			type Raw struct {
				Token *string `hcl:"token"`

				// We'll ignore anything else, to allow for future expansion.
				Remain hcl.Body `hcl:",remain"`
			}
			var raw Raw

			hostname, err := svchost.ForComparison(block.Labels[0])
			if err != nil {
				diags = diags.Append(tfdiags.Sourceless(
					tfdiags.Error,
					"Invalid hostname for credentials block",
					fmt.Sprintf("The hostname %q (given in %s) is not valid: %s.", hostname, fn, err),
				))
				continue
			}

			moreDiags := gohcl.DecodeBody(block.Body, ctx, &raw)
			diags = diags.Append(moreDiags)
			if moreDiags.HasErrors() {
				continue
			}

			creds := &Credentials{
				Host: hostname,
				Raw:  map[string]interface{}{},
			}
			if raw.Token != nil {
				creds.Raw["token"] = *raw.Token
			}
			result.Credentials = append(result.Credentials, creds)
		}
	}

	return result, diags
}

func loadConfigFileLegacyHCL(fn string, environ []string) (*configFile, tfdiags.Diagnostics) {
	// These is the structs we used to use in the main module to parse CLI
	// config files, so any existing stuff from here should be preserved
	// exactly as it used to be, modulo the normalization/reorganization we'll
	// do once we've run the parser.
	type ConfigHost struct {
		Services map[string]interface{} `hcl:"services"`
	}
	type ConfigCredentialsHelper struct {
		Args []string `hcl:"args"`
	}
	type LegacyConfig struct {
		Providers    map[string]string
		Provisioners map[string]string

		DisableCheckpoint          bool `hcl:"disable_checkpoint"`
		DisableCheckpointSignature bool `hcl:"disable_checkpoint_signature"`

		// If set, enables local caching of plugins in this directory to
		// avoid repeatedly re-downloading over the Internet.
		PluginCacheDir string `hcl:"plugin_cache_dir"`

		Hosts map[string]*ConfigHost `hcl:"host"`

		Credentials        map[string]map[string]interface{}   `hcl:"credentials"`
		CredentialsHelpers map[string]*ConfigCredentialsHelper `hcl:"credentials_helper"`
	}

	var diags tfdiags.Diagnostics
	result := &configFile{
		Filename: fn,
	}

	// Read the HCL file and prepare for parsing
	d, err := ioutil.ReadFile(fn)
	if err != nil {
		diags = diags.Append(fmt.Errorf("Error reading %s: %s", fn, err))
		return result, diags
	}

	// Parse it
	obj, err := legacyhcl.Parse(string(d))
	if err != nil {
		diags = diags.Append(fmt.Errorf("Error parsing %s: %s", fn, err))
		return result, diags
	}

	// Build up the result
	var tmp LegacyConfig
	if err := legacyhcl.DecodeObject(&tmp, obj); err != nil {
		diags = diags.Append(fmt.Errorf("Error parsing %s: %s", fn, err))
		return result, diags
	}

	// Replace all env vars, but only in the few places we historically have.
	// This capability has never applied to any other config items and we
	// don't intend to expand it here, since that would increase the
	// compaibility burden for the new parser.
	for k, v := range tmp.Providers {
		tmp.Providers[k] = os.Expand(v, makeGetenv(environ))
	}
	for k, v := range tmp.Provisioners {
		tmp.Provisioners[k] = os.Expand(v, makeGetenv(environ))
	}
	if tmp.PluginCacheDir != "" {
		tmp.PluginCacheDir = os.Expand(tmp.PluginCacheDir, makeGetenv(environ))
	}

	for k, v := range tmp.Providers {
		result.Providers = append(result.Providers, &LegacyPluginOverride{
			Name: k,
			Path: v,
		})
	}
	for k, v := range tmp.Provisioners {
		result.Provisioners = append(result.Provisioners, &LegacyPluginOverride{
			Name: k,
			Path: v,
		})
	}
	result.DisableCheckpoint = tmp.DisableCheckpoint
	result.DisableCheckpointSignature = tmp.DisableCheckpointSignature
	result.PluginCacheDir = tmp.PluginCacheDir
	for k, v := range tmp.Hosts {
		hostname, err := svchost.ForComparison(k)
		if err != nil {
			diags = diags.Append(tfdiags.Sourceless(
				// Ideally we'd use a source-ful diagnostic here, but this is
				// based on the old code that used to live in the main module
				// and it throws away the source location information by using
				// DecodeObject. We'd rather follow the old codepath as closely
				// as possible to minimize the risk of accidental differences.
				tfdiags.Error,
				"Invalid hostname for host block",
				fmt.Sprintf("The hostname %q (given in %s) is not valid: %s.", hostname, fn, err),
			))
			continue
		}
		result.Hosts = append(result.Hosts, &Host{
			Host:     hostname,
			Services: v.Services,
		})
	}
	for k, v := range tmp.Credentials {
		hostname, err := svchost.ForComparison(k)
		if err != nil {
			diags = diags.Append(tfdiags.Sourceless(
				tfdiags.Error,
				"Invalid hostname for credentials block",
				fmt.Sprintf("The hostname %q (given in %s) is not valid: %s.", hostname, fn, err),
			))
			continue
		}
		result.Credentials = append(result.Credentials, &Credentials{
			Host: hostname,
			Raw:  v,
		})
	}
	for k, v := range tmp.CredentialsHelpers {
		result.CredentialsHelpers = append(result.CredentialsHelpers, &CredentialsHelper{
			Type: k,
			Args: v.Args,
		})
	}

	return result, diags
}

func legacyHCLErrSubjectRange(filename string, err error) *hcl.Range {
	if pe, isPos := err.(*legacyhclparser.PosError); isPos {
		return legacyHCLPosRange(filename, pe.Pos).Ptr()
	}
	return nil
}

func legacyHCLPosRange(filename string, pos legacyhcltoken.Pos) hcl.Range {
	return hcl.Range{
		Filename: filename,
		Start: hcl.Pos{
			Line:   pos.Line,
			Column: pos.Column,
			Byte:   pos.Offset,
		},
		End: hcl.Pos{
			Line:   pos.Line,
			Column: pos.Column,
			Byte:   pos.Offset,
		},
	}
}

type fileFormat string

const (
	invalidFormat fileFormat = ""
	hclFormat     fileFormat = "hcl"
	jsonFormat    fileFormat = "json"
)

func configFileFormat(fn string) fileFormat {
	// Ignoring errors here because it is used only to indicate pattern
	// syntax errors, and our patterns are hard-coded here.
	hclMatched, _ := filepath.Match("*.tfrc", fn)
	jsonMatched, _ := filepath.Match("*.tfrc.json", fn)

	switch {
	case hclMatched:
		return hclFormat
	case jsonMatched:
		return jsonFormat
	default:
		return invalidFormat
	}
}
