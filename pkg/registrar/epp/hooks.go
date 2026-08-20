package epp

// nbio/xml must be used instead of encoding/xml because Go's stdlib marshaler drops
// namespace prefixes (domain:create → ns0:create), which EPP registrars reject.
// See: https://github.com/golang/go/issues/48821

import (
	nbioxml "github.com/nbio/xml"

	"github.com/riftcell/epp-test/pkg/registrar"
)

// BuildDomainCreateHook is called after the core domain:create XML is prepared.
// The hook receives the typed request and a *nbio/xml.Encoder positioned inside
// <epp:extension>; it writes registrar-specific extension elements into the encoder.
// Core domain XML is always produced by the generic adapter; the hook only extends it.
type BuildDomainCreateHook func(req registrar.DomainCreateRequest, ext *nbioxml.Encoder)

// BuildDomainUpdateHook is called after the core domain:update XML is prepared.
// The hook writes registrar-specific extension elements inside <epp:extension>.
type BuildDomainUpdateHook func(req registrar.DomainUpdateRequest, ext *nbioxml.Encoder)

// BuildContactCreateHook is called after the core contact:create XML is prepared.
// The hook writes registrar-specific extension elements inside <epp:extension>.
type BuildContactCreateHook func(req registrar.ContactCreateRequest, ext *nbioxml.Encoder)

// ParseDomainInfoHook is called after a domain:info response is parsed.
// The hook receives the raw <epp:resData> inner bytes and may populate
// result.Extensions with registrar-specific parsed data.
type ParseDomainInfoHook func(raw []byte, result *registrar.DomainResult)

// ParseContactInfoHook is called after a contact:info response is parsed.
// The hook receives the raw <epp:resData> inner bytes and may populate
// result.Extensions with registrar-specific parsed data.
type ParseContactInfoHook func(raw []byte, result *registrar.ContactResult)
