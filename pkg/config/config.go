// Package config loads and validates the epp-test.yaml configuration file.
// It uses Viper for YAML parsing and environment variable overrides.
//
// Configuration file discovery order:
//  1. $EPP_CONFIG_FILE (if set)
//  2. ./epp-test.yaml (current working directory)
//  3. $HOME/.epp-test/epp-test.yaml
//
// Environment variable overrides follow the convention:
//
//	EPP_REGISTRARS_<NAME>_<FIELD>
//
// where <NAME> is the registrar key in uppercase and <FIELD> is the field
// name in uppercase with underscores. Example:
//
//	EPP_REGISTRARS_INTERNETX_HOST overrides registrars.internetx.host
//
// Limitation: AutomaticEnv works for the four known registrar names
// (internetx, nicat, eurid, denic). For unknown names, supply a complete
// epp-test.yaml and override only credential fields via env vars.
package config

import (
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/viper"
)

// Config is the top-level configuration structure parsed from epp-test.yaml.
type Config struct {
	Registrars map[string]RegistrarConfig `mapstructure:"registrars"`
}

// RegistrarConfig holds all connection parameters for one registrar.
type RegistrarConfig struct {
	// Host is the EPP server hostname or IP address.
	Host string `mapstructure:"host"`
	// Port is the EPP server port (typically 700).
	Port int `mapstructure:"port"`
	// Username is the EPP login ID.
	Username string `mapstructure:"username"`
	// Password is the EPP login password. In CI, supply via EPP_REGISTRARS_<NAME>_PASSWORD.
	Password string `mapstructure:"password"`
	// CertFile is the path to the client TLS certificate PEM file (mutual TLS).
	CertFile string `mapstructure:"cert_file"`
	// KeyFile is the path to the client TLS private key PEM file (mutual TLS).
	KeyFile string `mapstructure:"key_file"`
	// CAFile is the path to the server CA certificate PEM file for verification.
	CAFile string `mapstructure:"ca_file"`
	// Extensions is the list of EPP service extension URIs to request on login.
	Extensions []string `mapstructure:"extensions"`
}

// Load reads the epp-test.yaml config file and environment variable overrides,
// unmarshals the result into a Config struct, and validates required fields.
//
// Returns an error with per-field messages if any required credential field is
// missing. Validation runs before any network connection is attempted.
func Load() (*Config, error) {
	v := viper.New()

	// Config file discovery.
	v.SetConfigName("epp-test")
	v.SetConfigType("yaml")
	v.AddConfigPath(".")
	v.AddConfigPath("$HOME/.epp-test")

	// Env var overrides: EPP_REGISTRARS_INTERNETX_HOST -> registrars.internetx.host
	// The replacer converts underscores in env var names to dots for nested key lookup.
	v.SetEnvPrefix("EPP")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	// Bind known registrar + field combinations explicitly so AutomaticEnv works
	// with the dynamic map key structure. These are the four supported registrars.
	// Explicit BindEnv is the documented workaround for Viper's dynamic map key
	// limitation with AutomaticEnv (see CONVENTIONS.md §9 "Config env override limitation").
	for _, name := range []string{"internetx", "nicat", "eurid", "denic"} {
		for _, field := range []string{"host", "port", "username", "password", "cert_file", "key_file", "ca_file"} {
			key := fmt.Sprintf("registrars.%s.%s", name, field)
			// BindEnv maps the Viper key to the env var name derived by the replacer:
			// dots -> underscores, prefixed with EPP_.
			// e.g. "registrars.internetx.host" -> "EPP_REGISTRARS_INTERNETX_HOST"
			_ = v.BindEnv(key) //nolint:errcheck // BindEnv only errors on empty key; key is always non-empty here
		}
	}

	if err := v.ReadInConfig(); err != nil {
		var notFound viper.ConfigFileNotFoundError
		if !errors.As(err, &notFound) {
			// A real read error (permission denied, invalid YAML) is fatal.
			return nil, fmt.Errorf("config: reading config file: %w", err)
		}
		// Config file not found is acceptable — pure env var config is valid for CI.
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("config: unmarshalling: %w", err)
	}

	if err := validate(&cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}
