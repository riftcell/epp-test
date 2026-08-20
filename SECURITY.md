# Security Policy

## Threat Model

The EPP Test Framework is a **development-time testing tool**, not a production service.

### What it protects

- **OT&E credentials** (EPP passwords, TLS client certificates) stored in `epp-test.yaml`
  config files. The framework reads these at test startup and never writes them to stdout,
  stderr, log files, or error messages.
- **Test output integrity** — no credential values appear in scenario results, JUnit XML,
  HTML reports, or Go test output.

### What it does not protect

- **Real EPP registry objects.** The framework is designed to target OT&E/sandbox environments
  only. Pointing it at a live production EPP endpoint is not a supported use case.
- **Network interception.** TLS between the test runner and the OT&E endpoint terminates at the
  registrar server. The framework does not add additional transport security beyond TLS 1.2.
- **Secrets committed to source control.** If `epp-test.yaml` with real credentials is committed
  to a repository, that is a misconfiguration — see Safe Usage Patterns below.

---

## Safe Usage Patterns for OT&E Credentials

**Never commit `epp-test.yaml` with real credentials.**

Use environment variables to supply sensitive fields in CI:

```bash
EPP_REGISTRARS_INTERNETX_PASSWORD=secret \
EPP_REGISTRARS_INTERNETX_USERNAME=user \
  go test -tags integration -run TestInternetX ./...
```

Environment variable format: `EPP_REGISTRARS_<NAME>_<FIELD>` where `<NAME>` is the registrar
key in uppercase and `<FIELD>` is the field name in uppercase. Example mappings:

| Env var | Config field |
|---------|-------------|
| `EPP_REGISTRARS_INTERNETX_HOST` | `registrars.internetx.host` |
| `EPP_REGISTRARS_INTERNETX_PASSWORD` | `registrars.internetx.password` |
| `EPP_REGISTRARS_DENIC_PASSWORD` | `registrars.denic.password` |

Add `epp-test.yaml` to your `.gitignore`:

```
epp-test.yaml
```

Commit only the example template (`configs/epp-test.example.yaml`) which contains no
real credentials.

---

## TLS Configuration

All EPP connections enforce a minimum of **TLS 1.2** (`tls.VersionTLS12`). This is set
in both the production adapter and the mock server:

| File | Setting |
|------|---------|
| `pkg/registrar/epp/connect.go:32` | `MinVersion: tls.VersionTLS12` |
| `pkg/mock/epp/server.go:92` | `MinVersion: tls.VersionTLS12` |

`InsecureSkipVerify` is **never set to `true`** in production code paths. References to
`InsecureSkipVerify` in the codebase are comments explaining why it must not be set, not
actual settings. This was confirmed by audit on 2026-06-29 (SEC-01).

Client certificate verification uses an `x509.CertPool` populated from the configured
`ca_file` field. The test path for mock servers uses `GenerateClientCert` to generate
ephemeral client certificates signed by a per-test CA — no shared long-lived test
certificates exist in the repository.

---

## Credential Hygiene

The DENIC RRI adapter transmits the login password as an **MD5 hex digest**, not as
plaintext. The hash is computed at the adapter boundary before calling `client.Login` —
the plaintext password value never appears on the wire or in any variable that could
reach a log statement.

Audit results (SEC-02, 2026-06-29):

- Zero `fmt.Sprint`/`log.*`/`t.Log` calls include password values in production code.
- The single `log.Printf` in `pkg/registrar/epp/connect.go` logs EPP service extension
  URI warnings only — no credential values.
- `pkg/registrar/rri/adapter.go:112` calls `a.client.Login(a.cfg.Username, md5Hex(a.cfg.Password))` —
  the MD5 hash is passed directly, not assigned to a variable or formatted into a string.

---

## Dependency Vulnerabilities

**govulncheck version:** v1.5.0
**Scan date:** 2026-08-20
**Go version at scan:** go1.25.13
**golang.org/x/net version at scan:** v0.56.0
**golang.org/x/text version at scan:** v0.39.0

**Result: No known vulnerabilities found.**

```
govulncheck ./...
No vulnerabilities found.
exit=0
```

At the 2026-06-29 baseline scan, 14 CVEs were identified and remediated:

- 13 stdlib CVEs (across `net/textproto`, `crypto/x509`, `crypto/tls`, `html/template`, `net`,
  `os`, `net/url`) — all fixed by upgrading the Go toolchain from go1.25 to go1.25.11.
- 1 CVE in `golang.org/x/net/idna` (GO-2026-5026, Punycode bypass) — fixed by upgrading
  `golang.org/x/net` from v0.32.0 to v0.55.0.

A 2026-08-20 follow-up scan found 6 additional CVEs disclosed after the baseline, all now
remediated:

- 4 stdlib CVEs — GO-2026-6091 (`html/template`, reached via `pkg/report/html.go`),
  GO-2026-6090 (`crypto/tls`, reached via the RRI/EPP TLS dial/read/write paths),
  GO-2026-5972 (`encoding/asn1`, reached via `pkg/mock/epp/tls.go` certificate generation),
  and GO-2026-5856 (`crypto/tls`, RRI/EPP TLS paths) — all fixed by raising the Go toolchain
  from go1.25.11 to go1.25.13.
- 1 CVE in `golang.org/x/text` — GO-2026-5970, reached via `NormalizeToACE` in
  `pkg/registrar/validate.go` — fixed by upgrading v0.37.0 -> v0.39.0.
- 1 CVE in `golang.org/x/net` — GO-2026-5942, a parser panic on invalid SVCB/HTTPS DNS
  records in `golang.org/x/net/dns/dnsmessage`, not reachable by this codebase's call
  graph but present in the required module — fixed by upgrading v0.55.0 -> v0.56.0.

No CVEs were accepted or deferred. All 20 (14 baseline + 6 follow-up) have been remediated.

---

## Responsible Disclosure

To report a security vulnerability in the EPP Test Framework:

1. **Email** the maintainer directly at **riftcell@gmail.com**.
2. Include a description of the vulnerability, steps to reproduce, and your assessment
   of the impact.
3. **Do not open a public GitHub issue** for security vulnerabilities — public disclosure
   before a fix is available puts users at risk.

A response will be sent within 5 business days. If you do not receive a response, follow
up by email. There is no formal bug bounty program.
