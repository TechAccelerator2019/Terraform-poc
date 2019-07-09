package cliconfig

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/hashicorp/terraform/svchost"
)

func TestLoadConfig(t *testing.T) {
	got, diags := LoadConfig([]string{"testdata/config"}, nil, nil)
	if diags.HasErrors() {
		t.Fatalf("unexpected error: %s", diags.Err().Error())
	}

	want := &Config{
		Providers: map[string]*LegacyPluginOverride{
			"aws": {
				Name: "aws",
				Path: "foo",
			},
			"do": {
				Name: "do",
				Path: "bar",
			},
		},
		Provisioners: map[string]*LegacyPluginOverride{},
		Hosts:        map[string]*Host{},
		Credentials:  map[string]*Credentials{},
	}

	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("wrong result\n%s", diff)
	}
}

func TestLoadConfig_env(t *testing.T) {
	got, diags := LoadConfig([]string{"testdata/config-env"}, nil, []string{"TFTEST=hello"})
	if diags.HasErrors() {
		t.Fatalf("unexpected error: %s", diags.Err().Error())
	}

	want := &Config{
		Providers: map[string]*LegacyPluginOverride{
			"aws": {
				Name: "aws",
				Path: "hello",
			},
			"google": {
				Name: "google",
				Path: "bar",
			},
		},
		Provisioners: map[string]*LegacyPluginOverride{
			"local": {
				Name: "local",
				Path: "hello",
			},
		},
		Hosts:       map[string]*Host{},
		Credentials: map[string]*Credentials{},
	}

	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("wrong result\n%s", diff)
	}
}

func TestLoadConfig_hosts(t *testing.T) {
	got, diags := LoadConfig([]string{"testdata/hosts"}, nil, nil)
	if diags.HasErrors() {
		t.Fatalf("unexpected error: %s", diags.Err().Error())
	}

	want := &Config{
		Providers:    map[string]*LegacyPluginOverride{},
		Provisioners: map[string]*LegacyPluginOverride{},
		Hosts: map[string]*Host{
			"example.com": {
				Host: svchost.Hostname("example.com"),
				Services: map[string]interface{}{
					"modules.v1": "https://example.com/",
				},
			},
		},
		Credentials: map[string]*Credentials{},
	}

	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("wrong result\n%s", diff)
	}
}

func TestLoadConfig_credentials(t *testing.T) {
	got, diags := LoadConfig([]string{"testdata/credentials"}, nil, nil)
	if diags.HasErrors() {
		t.Fatalf("unexpected error: %s", diags.Err().Error())
	}

	want := &Config{
		Providers:    map[string]*LegacyPluginOverride{},
		Provisioners: map[string]*LegacyPluginOverride{},
		Hosts:        map[string]*Host{},
		Credentials: map[string]*Credentials{
			"example.com": {
				Host: svchost.Hostname("example.com"),
				Raw: map[string]interface{}{
					"token": "foo the bar baz",
				},
			},
			"example.net": {
				Host: svchost.Hostname("example.net"),
				Raw: map[string]interface{}{
					"username": "foo",
					"password": "baz",
				},
			},
		},
		CredentialsHelper: &CredentialsHelper{
			Type: "foo",
			Args: []string{"bar", "baz"},
		},
	}

	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("wrong result\n%s", diff)
	}
}

/*
func TestConfigValidate(t *testing.T) {
	tests := map[string]struct {
		Config    *Config
		DiagCount int
	}{
		"nil": {
			nil,
			0,
		},
		"empty": {
			&Config{},
			0,
		},
		"host good": {
			&Config{
				Hosts: map[string]*ConfigHost{
					"example.com": {},
				},
			},
			0,
		},
		"host with bad hostname": {
			&Config{
				Hosts: map[string]*ConfigHost{
					"example..com": {},
				},
			},
			1, // host block has invalid hostname
		},
		"credentials good": {
			&Config{
				Credentials: map[string]map[string]interface{}{
					"example.com": map[string]interface{}{
						"token": "foo",
					},
				},
			},
			0,
		},
		"credentials with bad hostname": {
			&Config{
				Credentials: map[string]map[string]interface{}{
					"example..com": map[string]interface{}{
						"token": "foo",
					},
				},
			},
			1, // credentials block has invalid hostname
		},
		"credentials helper good": {
			&Config{
				CredentialsHelpers: map[string]*ConfigCredentialsHelper{
					"foo": {},
				},
			},
			0,
		},
		"credentials helper too many": {
			&Config{
				CredentialsHelpers: map[string]*ConfigCredentialsHelper{
					"foo": {},
					"bar": {},
				},
			},
			1, // no more than one credentials_helper block allowed
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			diags := test.Config.Validate()
			if len(diags) != test.DiagCount {
				t.Errorf("wrong number of diagnostics %d; want %d", len(diags), test.DiagCount)
				for _, diag := range diags {
					t.Logf("- %#v", diag.Description())
				}
			}
		})
	}
}

func TestConfig_Merge(t *testing.T) {
	c1 := &Config{
		Providers: map[string]string{
			"foo": "bar",
			"bar": "blah",
		},
		Provisioners: map[string]string{
			"local":  "local",
			"remote": "bad",
		},
		Hosts: map[string]*ConfigHost{
			"example.com": {
				Services: map[string]interface{}{
					"modules.v1": "http://example.com/",
				},
			},
		},
		Credentials: map[string]map[string]interface{}{
			"foo": {
				"bar": "baz",
			},
		},
		CredentialsHelpers: map[string]*ConfigCredentialsHelper{
			"buz": {},
		},
	}

	c2 := &Config{
		Providers: map[string]string{
			"bar": "baz",
			"baz": "what",
		},
		Provisioners: map[string]string{
			"remote": "remote",
		},
		Hosts: map[string]*ConfigHost{
			"example.net": {
				Services: map[string]interface{}{
					"modules.v1": "https://example.net/",
				},
			},
		},
		Credentials: map[string]map[string]interface{}{
			"fee": {
				"bur": "bez",
			},
		},
		CredentialsHelpers: map[string]*ConfigCredentialsHelper{
			"biz": {},
		},
	}

	expected := &Config{
		Providers: map[string]string{
			"foo": "bar",
			"bar": "baz",
			"baz": "what",
		},
		Provisioners: map[string]string{
			"local":  "local",
			"remote": "remote",
		},
		Hosts: map[string]*ConfigHost{
			"example.com": {
				Services: map[string]interface{}{
					"modules.v1": "http://example.com/",
				},
			},
			"example.net": {
				Services: map[string]interface{}{
					"modules.v1": "https://example.net/",
				},
			},
		},
		Credentials: map[string]map[string]interface{}{
			"foo": {
				"bar": "baz",
			},
			"fee": {
				"bur": "bez",
			},
		},
		CredentialsHelpers: map[string]*ConfigCredentialsHelper{
			"buz": {},
			"biz": {},
		},
	}

	actual := c1.Merge(c2)
	if !reflect.DeepEqual(actual, expected) {
		t.Fatalf("bad: %#v", actual)
	}
}

func TestConfig_Merge_disableCheckpoint(t *testing.T) {
	c1 := &Config{
		DisableCheckpoint: true,
	}

	c2 := &Config{}

	expected := &Config{
		Providers:         map[string]string{},
		Provisioners:      map[string]string{},
		DisableCheckpoint: true,
	}

	actual := c1.Merge(c2)
	if !reflect.DeepEqual(actual, expected) {
		t.Fatalf("bad: %#v", actual)
	}
}

func TestConfig_Merge_disableCheckpointSignature(t *testing.T) {
	c1 := &Config{
		DisableCheckpointSignature: true,
	}

	c2 := &Config{}

	expected := &Config{
		Providers:                  map[string]string{},
		Provisioners:               map[string]string{},
		DisableCheckpointSignature: true,
	}

	actual := c1.Merge(c2)
	if !reflect.DeepEqual(actual, expected) {
		t.Fatalf("bad: %#v", actual)
	}
}
*/
