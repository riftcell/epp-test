//go:build unit

package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/riftcell/epp-test/pkg/config"
)

// TestLoad verifies that Load() parses a valid YAML fixture correctly.
func TestLoad(t *testing.T) {
	// Change to the directory containing the example config so Viper's
	// AddConfigPath(".") finds configs/epp-test.example.yaml.
	// We copy the example file to a temp dir named epp-test.yaml for the test.
	dir := t.TempDir()
	src, err := os.ReadFile(filepath.Join("..", "..", "configs", "epp-test.example.yaml"))
	require.NoError(t, err, "reading example config fixture")
	dest := filepath.Join(dir, "epp-test.yaml")
	require.NoError(t, os.WriteFile(dest, src, 0o644))

	origDir, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(dir))
	t.Cleanup(func() { _ = os.Chdir(origDir) })

	cfg, err := config.Load()
	require.NoError(t, err)

	ix, ok := cfg.Registrars["internetx"]
	require.True(t, ok, "internetx registrar must be present")
	assert.Equal(t, "epp.internetx.de", ix.Host)
	assert.Equal(t, 700, ix.Port)
	assert.Equal(t, "test-user", ix.Username)
}

// TestLoadMissingFile verifies that Load() succeeds when no config file is found,
// returning an empty Config (pure env var config is valid for CI per D-05).
func TestLoadMissingFile(t *testing.T) {
	dir := t.TempDir()
	origDir, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(dir))
	t.Cleanup(func() { _ = os.Chdir(origDir) })

	cfg, err := config.Load()
	require.NoError(t, err, "missing config file must not return an error")
	assert.Empty(t, cfg.Registrars, "no registrars in env-only mode")
}

// TestValidationMissingHost verifies that a missing host produces a descriptive error.
func TestValidationMissingHost(t *testing.T) {
	dir := t.TempDir()
	yaml := []byte(`
registrars:
  test:
    port: 700
    username: user
    password: pass
`)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "epp-test.yaml"), yaml, 0o644))

	origDir, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(dir))
	t.Cleanup(func() { _ = os.Chdir(origDir) })

	_, err = config.Load()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "registrars.test.host: required")
}

// TestValidationMissingPort verifies that a missing port produces a descriptive error.
func TestValidationMissingPort(t *testing.T) {
	dir := t.TempDir()
	yaml := []byte(`
registrars:
  test:
    host: epp.example.com
    username: user
    password: pass
`)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "epp-test.yaml"), yaml, 0o644))

	origDir, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(dir))
	t.Cleanup(func() { _ = os.Chdir(origDir) })

	_, err = config.Load()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "registrars.test.port: required")
}

// TestValidationMissingUsername verifies that a missing username produces a descriptive error.
func TestValidationMissingUsername(t *testing.T) {
	dir := t.TempDir()
	yaml := []byte(`
registrars:
  test:
    host: epp.example.com
    port: 700
    password: pass
`)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "epp-test.yaml"), yaml, 0o644))

	origDir, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(dir))
	t.Cleanup(func() { _ = os.Chdir(origDir) })

	_, err = config.Load()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "registrars.test.username: required")
}

// TestValidationMissingPassword verifies that a missing password produces a descriptive error.
func TestValidationMissingPassword(t *testing.T) {
	dir := t.TempDir()
	yaml := []byte(`
registrars:
  test:
    host: epp.example.com
    port: 700
    username: user
`)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "epp-test.yaml"), yaml, 0o644))

	origDir, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(dir))
	t.Cleanup(func() { _ = os.Chdir(origDir) })

	_, err = config.Load()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "registrars.test.password: required")
}

// TestValidationValid verifies that a fully-specified registrar config passes validation.
func TestValidationValid(t *testing.T) {
	dir := t.TempDir()
	yaml := []byte(`
registrars:
  test:
    host: epp.example.com
    port: 700
    username: user
    password: pass
`)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "epp-test.yaml"), yaml, 0o644))

	origDir, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(dir))
	t.Cleanup(func() { _ = os.Chdir(origDir) })

	cfg, err := config.Load()
	require.NoError(t, err)
	assert.Equal(t, "epp.example.com", cfg.Registrars["test"].Host)
	assert.Equal(t, 700, cfg.Registrars["test"].Port)
}

// TestEnvOverride verifies that an env var overrides the value from the config file.
func TestEnvOverride(t *testing.T) {
	dir := t.TempDir()
	yaml := []byte(`
registrars:
  internetx:
    host: epp.internetx.de
    port: 700
    username: user
    password: pass
`)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "epp-test.yaml"), yaml, 0o644))

	origDir, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(dir))
	t.Cleanup(func() { _ = os.Chdir(origDir) })

	t.Setenv("EPP_REGISTRARS_INTERNETX_HOST", "override.example.com")

	cfg, err := config.Load()
	require.NoError(t, err)
	assert.Equal(t, "override.example.com", cfg.Registrars["internetx"].Host)
}
