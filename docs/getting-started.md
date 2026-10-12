# Getting Started

This guide walks you from a clean machine to a passing `go test -tags unit ./...` in under 10 minutes. No real registrar credentials are needed for unit tests — they run entirely against in-process mock servers.

## Prerequisites

- **Go 1.26.9 or later.** Check with `go version`. The framework uses generics and standard library features added in 1.20+; 1.26.9 is the minimum declared in `go.mod`.
- No other system dependencies. The framework compiles as a fully static binary (`CGO_ENABLED=0`). No C libraries, no database, no external runtime.

## Install / Module Setup

**Option A: Clone the repository**

```sh
git clone https://github.com/riftcell/epp-test
cd epp-test
```

**Option B: Add as a dependency to your own project**

```sh
go get github.com/riftcell/epp-test
```

## Config File

Unit tests use in-process mock servers and do not connect to any external endpoint, so credentials are ignored at the unit level. However, the `config.Load()` function validates that required fields are present when a YAML file is found. Copy the example config to avoid validation errors:

```sh
cp configs/epp-test.example.yaml epp-test.yaml
```

The example config (`configs/epp-test.example.yaml`) looks like this:

```yaml
# epp-test.yaml — EPP Test Framework Configuration
#
# Copy this file to epp-test.yaml and fill in your OT&E credentials.
# Never commit epp-test.yaml with real credentials — use environment variable
# overrides for CI:
#
#   EPP_REGISTRARS_INTERNETX_PASSWORD=secret go test -tags integration -run TestInternetX ./...
#
# Environment variable format:
#   EPP_REGISTRARS_<NAME>_<FIELD>
#   where <NAME> and <FIELD> are uppercase with underscores replacing dots.

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

Config file discovery order (first match wins):

1. `$EPP_CONFIG_FILE` — environment variable pointing to an absolute path
2. `./epp-test.yaml` — current working directory
3. `$HOME/.epp-test/epp-test.yaml` — user home directory

For the unit suite, credentials in the YAML file are not validated against any live server, so you can leave the example values as-is.

## Your First Test

Create a file `mytest_test.go` inside a package of this module with the following content. It starts an in-process mock EPP server, connects the generic EPP adapter to it over mutual TLS, and checks a domain:

```go
//go:build unit

package mytest_test

import (
    "context"
    "crypto/tls"
    "net"
    "strconv"
    "testing"

    "github.com/riftcell/epp-test/pkg/config"
    mockepp "github.com/riftcell/epp-test/pkg/mock/epp"
    "github.com/riftcell/epp-test/pkg/registrar/epp"
)

// The in-process mock is scripted: it sends exactly the frames the test queues,
// in order. InjectPoll queues the greeting; Expect queues one response per command.
const (
    greetingXML = `<?xml version="1.0" encoding="UTF-8"?>
<epp xmlns="urn:ietf:params:xml:ns:epp-1.0"><greeting><svID>Mock</svID><svDate>2026-01-01T00:00:00Z</svDate>
<svcMenu><version>1.0</version><lang>en</lang><objURI>urn:ietf:params:xml:ns:domain-1.0</objURI></svcMenu></greeting></epp>`
    loginOKXML = `<?xml version="1.0" encoding="UTF-8"?>
<epp xmlns="urn:ietf:params:xml:ns:epp-1.0"><response><result code="1000"><msg>Command completed successfully</msg></result>
<trID><svTRID>s-1</svTRID></trID></response></epp>`
    checkAvailXML = `<?xml version="1.0" encoding="UTF-8"?>
<epp xmlns="urn:ietf:params:xml:ns:epp-1.0"><response><result code="1000"><msg>Command completed successfully</msg></result>
<resData><domain:chkData xmlns:domain="urn:ietf:params:xml:ns:domain-1.0">
<domain:cd><domain:name avail="1">example.com</domain:name></domain:cd></domain:chkData></resData>
<trID><svTRID>s-2</svTRID></trID></response></epp>`
)

func TestCheckDomain(t *testing.T) {
    // Start an in-process mock EPP server (mutual TLS) on a random port.
    // t.Cleanup closes it automatically when the test ends.
    srv := mockepp.NewMockEPPServer(t)

    // Trust the mock's CA and present a client certificate it signed.
    clientCert, err := mockepp.GenerateClientCert(srv.CAKey, srv.CACert)
    if err != nil {
        t.Fatal(err)
    }
    tlsCfg := &tls.Config{Certificates: []tls.Certificate{clientCert}, RootCAs: srv.ClientPool, ServerName: "127.0.0.1"}

    host, portStr, _ := net.SplitHostPort(srv.Addr())
    port, _ := strconv.Atoi(portStr)
    cfg := config.RegistrarConfig{Host: host, Port: port, Username: "u", Password: "p"}

    reg, err := epp.NewGenericEPPAdapter("internetx", cfg, epp.WithTLSConfig(tlsCfg))
    if err != nil {
        t.Fatal(err)
    }
    t.Cleanup(func() { _ = reg.Close() })

    // Script the session: greeting, login response, check response.
    srv.InjectPoll([]byte(greetingXML))
    srv.Expect <- []byte(loginOKXML)
    srv.Expect <- []byte(checkAvailXML)

    ctx := context.Background()
    if err := reg.Login(ctx); err != nil {
        t.Fatalf("login: %v", err)
    }
    results, err := reg.CheckDomain(ctx, "example.com")
    if err != nil {
        t.Fatalf("check: %v", err)
    }
    if len(results) != 1 || results[0].Available == nil || !*results[0].Available {
        t.Fatalf("want example.com available, got %+v", results)
    }
}
```

Then run it:

```sh
go test -tags unit -run TestCheckDomain -v ./path/to/your/package
```

Expected output (trimmed):

```
--- PASS: TestCheckDomain (0.00s)
ok      github.com/riftcell/epp-test/...   0.006s
```

**The in-process mock is scripted.** It sends exactly the frames your test queues, in order: `InjectPoll` queues the greeting, and each value sent to `srv.Expect` answers one command. If the client sends a command with nothing queued, the call blocks until it times out, so queue one response per command.

To run a whole YAML scenario against the mock, queue one response per scenario step (including cleanup steps) and then call `runner.RunScenario(t, "domain_lifecycle.yaml", reg)`. [`scenarios/conformance/conformance_test.go`](../scenarios/conformance/conformance_test.go) does this for every conformance scenario and is the reference to copy from.

## Build Tags

The framework uses Go build tags to separate test layers:

| Command | What runs |
|---------|-----------|
| `go test ./...` | Compile-time assertions only (fastest, always offline) |
| `go test -tags unit ./...` | Full unit suite using in-process mock servers |
| `go test -tags integration ./...` | Integration suite against real OT&E endpoints |

See [CONVENTIONS.md](../CONVENTIONS.md) section 8 for the full build tag policy.

## Next Steps

- [How-To: Domains](how-to-domains.md) — domain lifecycle operations in detail
- [How-To: Contacts](how-to-contacts.md) — contact create, info, update, delete
- [How-To: Hosts](how-to-hosts.md) — host (nameserver) lifecycle
- [How-To: Polling](how-to-polling.md) — EPP message queue operations
- [How-To: DENIC RRI](how-to-denic-rri.md) — DENIC-specific RRI protocol
- [Config Reference](config-reference.md) — every config field documented
- [Architecture](architecture.md) — layered design and how to add a new registrar
