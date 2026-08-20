//go:build unit

package xml_test

import (
	"strings"
	"testing"

	eppxml "github.com/riftcell/epp-test/pkg/epp/xml"
	"github.com/nbio/xml"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestDomainCreateMarshalPrefix verifies that marshaling a DomainCreateCommand
// via github.com/nbio/xml produces the correct domain: namespace prefix and does
// NOT produce the ns0: prefix that encoding/xml would emit (Go issue #48821).
func TestDomainCreateMarshalPrefix(t *testing.T) {
	cmd := eppxml.DomainCreateCommand{
		Name:        "example.com",
		Registrant:  "REG1",
		Nameservers: []string{"ns1.example.com"},
		AuthInfo:    &eppxml.AuthInfo{PW: "abc123"},
		Period:      &eppxml.Period{Unit: "y", Value: 1},
	}

	out, err := xml.Marshal(cmd)
	require.NoError(t, err, "xml.Marshal should not fail on DomainCreateCommand")

	outStr := string(out)
	assert.True(t, strings.Contains(outStr, "domain:create"),
		"marshaled output must contain 'domain:create', got: %s", outStr)
	assert.False(t, strings.Contains(outStr, "ns0:"),
		"marshaled output must NOT contain 'ns0:' prefix (Go issue #48821), got: %s", outStr)
}

// TestEPPEnvelopeMarshalPrefixes verifies that marshaling a full EPPEnvelope wrapping
// a DomainCreateCommand produces both epp:epp and domain:create prefixes.
func TestEPPEnvelopeMarshalPrefixes(t *testing.T) {
	cmd := &eppxml.DomainCreateCommand{
		Name:       "example.com",
		Registrant: "CONTACT-1",
		AuthInfo:   &eppxml.AuthInfo{PW: "pw123"},
	}

	env := eppxml.EPPEnvelope{
		Command: &eppxml.EPPCommand{
			DomainCreate: cmd,
			ClTRID:       "test-0001",
		},
	}

	out, err := xml.Marshal(env)
	require.NoError(t, err, "xml.Marshal should not fail on EPPEnvelope")

	outStr := string(out)
	assert.True(t, strings.Contains(outStr, "epp:epp"),
		"envelope output must contain 'epp:epp', got: %s", outStr)
	assert.True(t, strings.Contains(outStr, "domain:create"),
		"envelope output must contain 'domain:create', got: %s", outStr)
	assert.False(t, strings.Contains(outStr, "ns0:"),
		"envelope output must NOT contain 'ns0:' prefix, got: %s", outStr)
}

// greetingFixture is a minimal but valid EPP greeting XML, as sent by a real server.
// SvID, version, lang, objURI, and svcExtension elements match RFC 5730 §2.4.
const greetingFixture = `<?xml version="1.0" encoding="UTF-8"?>
<epp:greeting xmlns:epp="urn:ietf:params:xml:ns:epp-1.0">
  <epp:svID>Example EPP Server</epp:svID>
  <epp:svcMenu>
    <epp:version>1.0</epp:version>
    <epp:lang>en</epp:lang>
    <epp:objURI>urn:ietf:params:xml:ns:domain-1.0</epp:objURI>
    <epp:objURI>urn:ietf:params:xml:ns:contact-1.0</epp:objURI>
    <epp:objURI>urn:ietf:params:xml:ns:host-1.0</epp:objURI>
    <epp:svcExtension>
      <epp:extURI>urn:ietf:params:xml:ns:rgp-1.0</epp:extURI>
      <epp:extURI>urn:ietf:params:xml:ns:secDNS-1.1</epp:extURI>
    </epp:svcExtension>
  </epp:svcMenu>
</epp:greeting>`

// TestGreetingUnmarshal verifies that unmarshaling a server greeting XML fixture
// into the Greeting struct correctly populates the SvcMenu fields.
func TestGreetingUnmarshal(t *testing.T) {
	var g eppxml.Greeting
	err := xml.Unmarshal([]byte(greetingFixture), &g)
	require.NoError(t, err, "xml.Unmarshal should not fail on greeting fixture")

	require.NotEmpty(t, g.SvcMenu.Version, "SvcMenu.Version must not be empty")
	assert.Contains(t, g.SvcMenu.Version, "1.0",
		"SvcMenu.Version must contain '1.0'")

	require.NotNil(t, g.SvcMenu.SvcExtension, "SvcMenu.SvcExtension must not be nil")
	require.NotEmpty(t, g.SvcMenu.SvcExtension.ExtURI,
		"SvcMenu.SvcExtension.ExtURI must not be empty")
}

// chkDataFixture is a minimal domain:chkData XML response body, as returned inside
// <epp:resData> for a domain:check command.
const chkDataFixture = `<domain:chkData xmlns:domain="urn:ietf:params:xml:ns:domain-1.0">
  <domain:cd>
    <domain:name avail="1">example.com</domain:name>
  </domain:cd>
  <domain:cd>
    <domain:name avail="0">taken.com</domain:name>
  </domain:cd>
</domain:chkData>`

// TestDomainCheckResponseUnmarshal verifies that unmarshaling a domain:chkData fixture
// into DomainCheckResponse correctly parses the avail attribute as a bool.
func TestDomainCheckResponseUnmarshal(t *testing.T) {
	var resp eppxml.DomainCheckResponse
	err := xml.Unmarshal([]byte(chkDataFixture), &resp)
	require.NoError(t, err, "xml.Unmarshal should not fail on chkData fixture")

	require.Len(t, resp.Results, 2, "expected 2 check results")

	assert.Equal(t, "example.com", resp.Results[0].Name.Value)
	assert.True(t, resp.Results[0].Name.Avail,
		"example.com should be available (avail=1)")

	assert.Equal(t, "taken.com", resp.Results[1].Name.Value)
	assert.False(t, resp.Results[1].Name.Avail,
		"taken.com should not be available (avail=0)")
}
