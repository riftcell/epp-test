// Package epp provides GenericEPPAdapter, the persistent-TLS-connection EPP client
// that implements the registrar.Registrar interface. It carries nil-safe, function-field
// extension hooks (D-01, D-02) so registrar-specific constructors (InternetX, NiCAT,
// EURid, DENIC) can inject extension XML for hookable operations without modifying the
// generic adapter core.
//
// Connection lifecycle: Login(ctx) dials TLS, reads the server greeting, and sends
// epp:login. All subsequent operations reuse the connection behind a sync.Mutex. If a
// mid-operation network error occurs, the adapter reconnects transparently, re-reads the
// greeting, re-logs in, and retries the failed operation once (D-04). Ping(ctx) sends an
// EPP hello and reads the greeting response to verify liveness (D-05).
//
// Extension hook design: Each hookable operation has a corresponding function field on
// GenericEPPAdapter that is nil by default. Build hooks receive the typed request and a
// positioned *nbio/xml.Encoder and write extension elements into <epp:extension>. Parse
// hooks receive the raw resData bytes and may populate result.Extensions. All hook call
// sites nil-check before calling — nil hooks are no-ops that never panic (REG-01).
//
// The adapter marshals all commands using github.com/nbio/xml (not encoding/xml) so that
// EPP namespace prefixes (domain:create, contact:info, etc.) are preserved on the wire.
// Go's stdlib encoding/xml drops these prefixes (see github.com/golang/go/issues/48821),
// causing real EPP servers to reject the frames.
package epp
