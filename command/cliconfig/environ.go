package cliconfig

import (
	"strings"

	"github.com/zclconf/go-cty/cty"
)

func environConfig(environ []string) *configFile {
	const pluginCacheDirEnvVar = "TF_PLUGIN_CACHE_DIR"

	result := &configFile{
		Filename: "<environment>",
	}

	if d := getEnv(environ, pluginCacheDirEnvVar); d != "" {
		result.PluginCacheDir = d
	}

	return result
}

func getEnv(environ []string, name string) string {
	for _, v := range environ {
		if len(v) <= len(name) {
			continue
		}
		if strings.HasPrefix(v, name) && v[len(name)] == '=' {
			return v[len(name)+1:]
		}
	}
	return ""
}

func makeGetenv(environ []string) func(string) string {
	return func(name string) string {
		return getEnv(environ, name)
	}
}

func environForHCL(environ []string) map[string]cty.Value {
	ret := make(map[string]cty.Value, len(environ))
	for _, e := range environ {
		eq := strings.IndexByte(e, '=')
		if eq == -1 {
			continue
		}
		ret[e[:eq]] = cty.StringVal(e[eq+1:])
	}
	return ret
}
