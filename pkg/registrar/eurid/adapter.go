package eurid

import (
	"fmt"

	nbioxml "github.com/nbio/xml"

	"github.com/riftcell/epp-test/pkg/config"
	"github.com/riftcell/epp-test/pkg/registrar"
	epp "github.com/riftcell/epp-test/pkg/registrar/epp"
)

// Compile-time assertion: EURidAdapter must implement registrar.Registrar.
// Embedding GenericEPPAdapter promotes all Registrar methods.
var _ registrar.Registrar = (*EURidAdapter)(nil)

// EURid extension namespace URIs (D-08, REG-04).
// Five namespaces are known from REQUIREMENTS.md; OT&E discovery will populate
// real field content without changing the hook API.
const (
	contactExtNS = "urn:ietf:params:xml:ns:contact-ext-1.3" // contact-ext-1.3
	domainExtNS  = "urn:ietf:params:xml:ns:domain-ext-2.4"  // domain-ext-2.4
	nsgroupNS    = "urn:ietf:params:xml:ns:nsgroup-1.1"     // nsgroup-1.1
	homoglyphNS  = "urn:ietf:params:xml:ns:homoglyph-1.0"   // homoglyph-1.0
	dnsQualityNS = "urn:ietf:params:xml:ns:dnsQuality-2.0"  // dnsQuality-2.0
)

// EURidAdapter is the EURid (.eu) EPP registrar adapter (REG-04).
//
// It wraps GenericEPPAdapter and wires stub hooks for EURid's five extension
// namespaces. The hooks are STUBS for Phase 3 — OT&E discovery will fill
// real field content without changing the hook API (D-08).
type EURidAdapter struct {
	*epp.GenericEPPAdapter
}

// NewEURidAdapter creates a EURid EPP adapter with stub hooks for all five
// EURid extension namespaces wired.
//
// Options are passed through to GenericEPPAdapter — unit tests use WithTLSConfig
// to inject the mock server's CA pool and client certificate.
func NewEURidAdapter(cfg config.RegistrarConfig, opts ...epp.Option) (*EURidAdapter, error) {
	g, err := epp.NewGenericEPPAdapter("eurid", cfg, opts...)
	if err != nil {
		return nil, fmt.Errorf("eurid adapter: %w", err)
	}
	g.OnBuildContactCreate = buildContactExt // contact-ext-1.3 (stub)
	g.OnBuildDomainCreate = buildDomainExt   // domain-ext-2.4 + nsgroup-1.1 (stub)
	g.OnParseDomainInfo = parseDomainExt     // homoglyph-1.0 + dnsQuality-2.0 (stub)
	return &EURidAdapter{g}, nil
}

// buildContactExt is the OnBuildContactCreate hook for EURid.
//
// Phase 3 stub: writes a self-closing <contact-ext:create> element for
// the contact-ext-1.3 namespace. Real field content (e.g., naturalPerson,
// whoisEmail, vatNo) is populated from OT&E discovery (REG-04).
func buildContactExt(req registrar.ContactCreateRequest, enc *nbioxml.Encoder) {
	// req fields will be read by the real implementation; unused in this stub.
	_ = req

	// contact-ext-1.3 stub element.
	start := nbioxml.StartElement{
		Name: nbioxml.Name{
			Space: contactExtNS,
			Local: "create",
		},
		Attr: []nbioxml.Attr{
			{Name: nbioxml.Name{Local: "xmlns:contact-ext"}, Value: contactExtNS},
		},
	}
	_ = enc.EncodeToken(start)       //nolint:errcheck // hook writes to in-memory encoder; errors are deferred to enc.Flush(), see CONVENTIONS.md §3
	_ = enc.EncodeToken(start.End()) //nolint:errcheck // hook writes to in-memory encoder; errors are deferred to enc.Flush(), see CONVENTIONS.md §3
	_ = enc.Flush()                  //nolint:errcheck // hook best-effort flush; calling adapter propagates any XML error via Encode result
}

// buildDomainExt is the OnBuildDomainCreate hook for EURid.
//
// Phase 3 stub: writes self-closing stub elements for domain-ext-2.4 and
// nsgroup-1.1 namespaces. Real field content (e.g., registrantType, nsgroup
// handles) is populated from OT&E discovery (REG-04).
func buildDomainExt(req registrar.DomainCreateRequest, enc *nbioxml.Encoder) {
	// req fields will be read by the real implementation; unused in this stub.
	_ = req

	// domain-ext-2.4 stub element.
	domStart := nbioxml.StartElement{
		Name: nbioxml.Name{
			Space: domainExtNS,
			Local: "create",
		},
		Attr: []nbioxml.Attr{
			{Name: nbioxml.Name{Local: "xmlns:domain-ext"}, Value: domainExtNS},
		},
	}
	_ = enc.EncodeToken(domStart)       //nolint:errcheck // hook writes to in-memory encoder; errors are deferred to enc.Flush(), see CONVENTIONS.md §3
	_ = enc.EncodeToken(domStart.End()) //nolint:errcheck // hook writes to in-memory encoder; errors are deferred to enc.Flush(), see CONVENTIONS.md §3

	// nsgroup-1.1 stub element.
	nsgStart := nbioxml.StartElement{
		Name: nbioxml.Name{
			Space: nsgroupNS,
			Local: "create",
		},
		Attr: []nbioxml.Attr{
			{Name: nbioxml.Name{Local: "xmlns:nsgroup"}, Value: nsgroupNS},
		},
	}
	_ = enc.EncodeToken(nsgStart)       //nolint:errcheck // hook writes to in-memory encoder; errors are deferred to enc.Flush(), see CONVENTIONS.md §3
	_ = enc.EncodeToken(nsgStart.End()) //nolint:errcheck // hook writes to in-memory encoder; errors are deferred to enc.Flush(), see CONVENTIONS.md §3

	_ = enc.Flush() //nolint:errcheck // hook best-effort flush; calling adapter propagates any XML error via Encode result
}

// parseDomainExt is the OnParseDomainInfo hook for EURid.
//
// Phase 3 stub: leaves result.Extensions as-is. Real field content
// (homoglyph group info from homoglyph-1.0, DNS quality metrics from
// dnsQuality-2.0) is populated from OT&E discovery (REG-04).
//
// Namespaces parsed by the real implementation:
//   - homoglyph-1.0  (urn:ietf:params:xml:ns:homoglyph-1.0)
//   - dnsQuality-2.0 (urn:ietf:params:xml:ns:dnsQuality-2.0)
func parseDomainExt(raw []byte, result *registrar.DomainResult) {
	// raw bytes will be parsed by the real implementation to populate
	// result.Extensions with homoglyph and DNS quality fields.
	_ = raw
	// Stub: leave result.Extensions unchanged; an empty map is acceptable.
	if result.Extensions == nil {
		result.Extensions = map[string]any{}
	}
}
