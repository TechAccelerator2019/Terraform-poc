package cliconfig

// LegacyPluginOverride represents an entry from either the "providers" or
// "provisioners" maps in the CLI configuration, both of which are deprecated
// in favor of placing plugin executables directly in one of the discovery
// search paths.
type LegacyPluginOverride struct {
	Name string
	Path string
}
