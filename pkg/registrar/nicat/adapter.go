package nicat

import (
	"fmt"

	nbioxml "github.com/nbio/xml"

	"github.com/riftcell/epp-test/pkg/config"
	"github.com/riftcell/epp-test/pkg/registrar"
	epp "github.com/riftcell/epp-test/pkg/registrar/epp"
)

// Compile-time assertion: NiCATAdapter must implement registrar.Registrar.
// Embedding GenericEPPAdapter promotes all Registrar methods.
var _ registrar.Registrar = (*NiCATAdapter)(nil)

// atExtVerificationNS is the at-ext-verification-1.0 namespace URI (REG-03, D-08).
const atExtVerificationNS = "urn:ietf:params:xml:ns:at-ext-verification-1.0"

// NiCATAdapter is the NiC.AT (.at) EPP registrar adapter (REG-03).
//
// It wraps GenericEPPAdapter and wires the OnBuildContactCreate hook to inject
// the at-ext-verification-1.0 contact verification extension on every
// contact:create command. The hook signature is final; OT&E discovery will
// populate real verification field values without changing the API (D-08).
type NiCATAdapter struct {
	*epp.GenericEPPAdapter
}

// NewNiCATAdapter creates a NiC.AT EPP adapter with the at-ext-verification-1.0
// contact-create hook wired.
//
// Options are passed through to GenericEPPAdapter — unit tests use WithTLSConfig
// to inject the mock server's CA pool and client certificate.
func NewNiCATAdapter(cfg config.RegistrarConfig, opts ...epp.Option) (*NiCATAdapter, error) {
	g, err := epp.NewGenericEPPAdapter("nicat", cfg, opts...)
	if err != nil {
		return nil, fmt.Errorf("nicat adapter: %w", err)
	}
	g.OnBuildContactCreate = buildVerificationExt
	return &NiCATAdapter{g}, nil
}

// buildVerificationExt is the OnBuildContactCreate hook for NiC.AT.
//
// It writes a self-closing <ax:create> element into the extension encoder.
// This is a STUB for Phase 3 — the real implementation will read verification
// fields from req (e.g., req.Extensions["ax:verificationStatus"]) and encode
// them per the at-ext-verification-1.0 schema discovered during OT&E.
//
// The req parameter is currently unused by the stub; the comment above documents
// that the real implementation will consume req fields.
func buildVerificationExt(req registrar.ContactCreateRequest, enc *nbioxml.Encoder) {
	// Phase 3 stub: encode a self-closing extension element to prove the hook
	// is wired and fires without panic. Real field content (verification status,
	// verification date, verification source) comes from OT&E discovery (REG-03).
	//
	// req is unused in this stub; the real implementation reads:
	//   req.Extensions["ax:verificationStatus"] — e.g., "Identified"
	//   req.Extensions["ax:verificationDate"]   — e.g., "2026-01-15"
	_ = req

	start := nbioxml.StartElement{
		Name: nbioxml.Name{
			Space: atExtVerificationNS,
			Local: "create",
		},
		Attr: []nbioxml.Attr{
			{
				Name:  nbioxml.Name{Local: "xmlns:ax"},
				Value: atExtVerificationNS,
			},
		},
	}
	_ = enc.EncodeToken(start)       //nolint:errcheck // hook writes to in-memory encoder; errors are deferred to enc.Flush(), see CONVENTIONS.md §3
	_ = enc.EncodeToken(start.End()) //nolint:errcheck // hook writes to in-memory encoder; errors are deferred to enc.Flush(), see CONVENTIONS.md §3
	_ = enc.Flush()                  //nolint:errcheck // hook best-effort flush; calling adapter propagates any XML error via Encode result
}
