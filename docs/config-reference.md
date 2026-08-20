# Config Reference

This reference documents every field in `RegistrarConfig` (defined in `pkg/config/config.go`), its YAML key, Go type, environment variable override, whether it is required, its default value, and validation rules.

See [Getting Started](getting-started.md) for how to create and locate the config file. See [Architecture](architecture.md) for how config is consumed by adapters.

## Config File Discovery

`config.Load()` searches for `epp-test.yaml` in the following order (first match wins):

| Priority | Location |
|----------|----------|
| 1 | `$EPP_CONFIG_FILE` — absolute path in environment variable |
| 2 | `./epp-test.yaml` — current working directory |
| 3 | `$HOME/.epp-test/epp-test.yaml` — user home directory |

If no config file is found and no environment variables are set, `Load()` returns without error — but subsequent validation will fail if any registered registrar block is missing required fields.

## Top-Level Structure

```yaml
registrars:
  internetx:    # registrar key (see "Registrar Keys" below)
    host: ...
    port: ...
    # ...
  nicat:
    # ...
```

The top-level key is `registrars`. Its value is a map from registrar key to `RegistrarConfig`.

## RegistrarConfig Fields

| Field (YAML key) | Type | Env Var | Required | Default | Validation / Notes |
|------------------|------|---------|----------|---------|-------------------|
| `host` | `string` | `EPP_REGISTRARS_<NAME>_HOST` | Yes | — | Must be non-empty; hostname or IP of the EPP/RRI server |
| `port` | `int` | `EPP_REGISTRARS_<NAME>_PORT` | Yes | — | Must be non-zero; typically 700 for EPP |
| `username` | `string` | `EPP_REGISTRARS_<NAME>_USERNAME` | Yes | — | EPP login ID; supplied by the registrar |
| `password` | `string` | `EPP_REGISTRARS_<NAME>_PASSWORD` | Yes | — | EPP login password. DENIC: transmitted as MD5 hash (handled by adapter) |
| `cert_file` | `string` | `EPP_REGISTRARS_<NAME>_CERT_FILE` | No | `""` | Path to client TLS certificate PEM file (mutual TLS). Optional; omit if registrar does not require mTLS |
| `key_file` | `string` | `EPP_REGISTRARS_<NAME>_KEY_FILE` | No | `""` | Path to client TLS private key PEM file. Must be set if `cert_file` is set |
| `ca_file` | `string` | `EPP_REGISTRARS_<NAME>_CA_FILE` | No | `""` | Path to server CA certificate PEM file. Optional; if omitted, the system certificate pool is used |
| `extensions` | `[]string` | — | No | `[]` | List of EPP service extension URIs requested on `LOGIN`. Not overridable via env var (list type); set in YAML only |

`<NAME>` in env var names is the registrar key in uppercase (e.g., `internetx` → `INTERNETX`).

### Required Fields

`host`, `port`, `username`, and `password` are required. If any of these are missing for a registrar block that appears in the config, `Load()` returns a validation error before any network connection is attempted.

### Validation Error Format

Validation errors follow the format `registrars.<name>.<field>: required`. Multiple missing fields are joined into a single error via `errors.Join`:

```text
registrars.internetx.host: required
registrars.internetx.password: required
```

## Registrar Keys

The four supported registrar keys are:

| Key | Registrar | Default Port |
|-----|-----------|-------------|
| `internetx` | InternetX EPP | 700 |
| `nicat` | NiCAT (nic.at) EPP | 700 |
| `eurid` | EURid EPP | 700 |
| `denic` | DENIC RRI (via `DENICAdapter`) | 700 |

## Environment Variable Convention

Environment variable overrides follow the pattern:

```
EPP_REGISTRARS_<NAME>_<FIELD>
```

where `<NAME>` is the registrar key in uppercase and `<FIELD>` is the YAML field name in uppercase with underscores. Examples:

```sh
EPP_REGISTRARS_INTERNETX_HOST=epp.internetx.de
EPP_REGISTRARS_INTERNETX_PASSWORD=prod-secret
EPP_REGISTRARS_NICAT_CERT_FILE=/run/secrets/nicat-client.pem
EPP_REGISTRARS_DENIC_USERNAME=denic-registrar
```

### Known Registrar Limitation

Viper's `AutomaticEnv` works for statically known struct keys. For the `map[string]RegistrarConfig` structure keyed by registrar name, env var overrides require explicit `BindEnv` calls per field. The four registrar names above are explicitly bound in `config.Load()`.

**For unknown registrar keys** (e.g., a fifth custom registrar): supply a complete `epp-test.yaml` with all fields populated. You can override credential fields via `EPP_REGISTRARS_<NAME>_<FIELD>` only if you add explicit `BindEnv` calls for that key in `pkg/config/config.go`.

## Worked Example

### Full YAML Config

```yaml
# epp-test.yaml
# Copy from configs/epp-test.example.yaml and fill in your OT&E credentials.
# Do NOT commit this file with real credentials.

registrars:
  internetx:
    host: epp.internetx.de
    port: 700
    username: test-user
    password: secret
    cert_file: /certs/internetx/client.pem
    key_file: /certs/internetx/client.key
    ca_file: /certs/internetx/ca.pem
    extensions:
      - urn:ietf:params:xml:ns:domain-1.0
      - urn:ietf:params:xml:ns:contact-1.0
      - urn:ietf:params:xml:ns:host-1.0

  nicat:
    host: epp.nic.at
    port: 700
    username: nic-user
    password: nic-secret
    cert_file: /certs/nicat/client.pem
    key_file: /certs/nicat/client.key
    extensions:
      - http://www.nic.at/xsd/at-ext-verification-1.0

  eurid:
    host: epp.eurid.eu
    port: 700
    username: eurid-user
    password: eurid-secret
    extensions:
      - http://www.eurid.eu/xml/epp/contact-ext-1.3
      - http://www.eurid.eu/xml/epp/domain-ext-2.4

  denic:
    host: epp.denic.de
    port: 700
    username: denic-user
    password: denic-secret
    extensions: []
```

### CI: Environment Variables Only

For CI pipelines, supply credentials via environment variables and keep a minimal YAML in the repository (without credentials):

```sh
# Minimal epp-test.yaml checked into the repo (no credentials):
#   registrars:
#     internetx:
#       host: epp.internetx.de
#       port: 700
#       username: ci-user
#       password: placeholder   # overridden by env var below

# CI invocation:
EPP_REGISTRARS_INTERNETX_PASSWORD=real-secret \
EPP_REGISTRARS_INTERNETX_CERT_FILE=/run/secrets/internetx-client.pem \
EPP_REGISTRARS_INTERNETX_KEY_FILE=/run/secrets/internetx-client.key \
  go test -tags integration -run TestInternetX ./...
```

The `EPP_CONFIG_FILE` variable can point to a config file in a secrets volume if the default discovery path is not suitable:

```sh
EPP_CONFIG_FILE=/run/secrets/epp-test.yaml \
  go test -tags integration ./...
```
