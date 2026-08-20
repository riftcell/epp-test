package xml

import "github.com/nbio/xml"

// Domain namespace URI (RFC 5731).
const domainNS = "urn:ietf:params:xml:ns:domain-1.0"

// DomainCheckCommand is the EPP domain:check wire frame for checking name availability.
type DomainCheckCommand struct {
	XMLName xml.Name `xml:"urn:ietf:params:xml:ns:domain-1.0 domain:check"`
	// Names is the list of fully qualified domain names to check.
	Names []string `xml:"domain:name"`
}

// DomainCreateCommand is the EPP domain:create wire frame for registering a domain.
type DomainCreateCommand struct {
	XMLName xml.Name `xml:"urn:ietf:params:xml:ns:domain-1.0 domain:create"`
	// Name is the fully qualified domain name to register.
	Name string `xml:"domain:name"`
	// Period specifies the registration length.
	Period *Period `xml:"domain:period,omitempty"`
	// Nameservers lists the host objects to delegate the domain to.
	Nameservers []string `xml:"domain:ns>domain:hostObj,omitempty"`
	// Registrant is the contact ID of the domain registrant.
	Registrant string `xml:"domain:registrant"`
	// AuthInfo holds the authorization code for the domain.
	AuthInfo *AuthInfo `xml:"domain:authInfo"`
}

// Period specifies the registration or renewal duration.
type Period struct {
	XMLName xml.Name `xml:"domain:period"`
	// Unit is the time unit: "y" for years (default) or "m" for months.
	Unit string `xml:"unit,attr"`
	// Value is the numeric length in the specified unit.
	Value int `xml:",chardata"`
}

// AuthInfo holds the domain authorization code used for transfers and creates.
type AuthInfo struct {
	XMLName xml.Name `xml:"domain:authInfo"`
	// PW is the authorization password string.
	PW string `xml:"domain:pw"`
}

// DomainInfoName wraps the domain name element in a domain:info command so the
// optional hosts attribute can be carried alongside the name value.
type DomainInfoName struct {
	// Value is the fully qualified domain name.
	Value string `xml:",chardata"`
	// Hosts controls which host objects are returned: "all", "sub", "del", or "none".
	Hosts string `xml:"hosts,attr,omitempty"`
}

// DomainInfoCommand is the EPP domain:info wire frame for retrieving a domain object.
type DomainInfoCommand struct {
	XMLName xml.Name `xml:"urn:ietf:params:xml:ns:domain-1.0 domain:info"`
	// Name wraps the domain name with an optional hosts attribute.
	Name DomainInfoName `xml:"domain:name"`
}

// DomainUpdateCommand is the EPP domain:update wire frame for modifying a domain.
type DomainUpdateCommand struct {
	XMLName xml.Name `xml:"urn:ietf:params:xml:ns:domain-1.0 domain:update"`
	// Name is the domain to update.
	Name string `xml:"domain:name"`
	// Add specifies nameservers and statuses to add.
	Add *DomainUpdateAddRem `xml:"domain:add,omitempty"`
	// Rem specifies nameservers and statuses to remove.
	Rem *DomainUpdateAddRem `xml:"domain:rem,omitempty"`
}

// DomainUpdateAddRem holds nameservers and statuses to add or remove in a domain update.
type DomainUpdateAddRem struct {
	// Nameservers are host objects to add or remove from the domain delegation.
	Nameservers []string `xml:"domain:ns>domain:hostObj,omitempty"`
	// Statuses are EPP status values to add or remove.
	Statuses []DomainStatus `xml:"domain:status,omitempty"`
}

// DomainStatus represents an EPP status element with an s attribute.
type DomainStatus struct {
	// S is the status value string (e.g., "clientHold", "ok").
	S string `xml:"s,attr"`
}

// DomainDeleteCommand is the EPP domain:delete wire frame for deleting a domain.
type DomainDeleteCommand struct {
	XMLName xml.Name `xml:"urn:ietf:params:xml:ns:domain-1.0 domain:delete"`
	// Name is the fully qualified domain name to delete.
	Name string `xml:"domain:name"`
}

// DomainRenewCommand is the EPP domain:renew wire frame for extending a domain registration.
type DomainRenewCommand struct {
	XMLName xml.Name `xml:"urn:ietf:params:xml:ns:domain-1.0 domain:renew"`
	// Name is the domain to renew.
	Name string `xml:"domain:name"`
	// CurExpDate is the current expiry date in YYYY-MM-DD format, required by RFC 5731.
	CurExpDate string `xml:"domain:curExpDate"`
	// Period specifies the renewal duration.
	Period *Period `xml:"domain:period,omitempty"`
}

// DomainTransferCommand is the inner domain:transfer element carrying the domain name
// and optional auth info. The Op attribute (request/approve/cancel/reject/query) is
// carried by the enclosing DomainTransfer wrapper.
type DomainTransferCommand struct {
	XMLName xml.Name `xml:"urn:ietf:params:xml:ns:domain-1.0 domain:transfer"`
	// Name is the domain subject to the transfer operation.
	Name string `xml:"domain:name"`
	// AuthInfo is required for "request" operations; omitted for others.
	AuthInfo *AuthInfo `xml:"domain:authInfo,omitempty"`
}

// DomainTransfer wraps a DomainTransferCommand with the transfer op attribute.
// Op must be one of "request", "approve", "cancel", "reject", or "query".
// The EPPCommand.DomainTransfer field holds this wrapper directly.
type DomainTransfer struct {
	XMLName xml.Name `xml:"urn:ietf:params:xml:ns:epp-1.0 epp:transfer"`
	// Op is the transfer operation code placed as an attribute on <epp:transfer>.
	Op      string                `xml:"op,attr"`
	Command DomainTransferCommand `xml:""`
}

// DomainInfoResponse is parsed from the <domain:infData> element in <epp:resData>
// for domain:info responses.
type DomainInfoResponse struct {
	XMLName xml.Name `xml:"urn:ietf:params:xml:ns:domain-1.0 domain:infData"`
	// Name is the fully qualified domain name.
	Name string `xml:"domain:name"`
	// ROID is the registry object identifier.
	ROID string `xml:"domain:roid"`
	// Statuses lists the EPP status codes on the domain.
	Statuses []DomainStatus `xml:"domain:status"`
	// Registrant is the contact ID of the domain registrant.
	Registrant string `xml:"domain:registrant"`
	// CrDate is the creation date in RFC 3339 format.
	CrDate string `xml:"domain:crDate"`
	// ExDate is the expiry date in RFC 3339 format.
	ExDate string `xml:"domain:exDate"`
}

// DomainCheckResult is one result element within a domain:chkData response.
type DomainCheckResult struct {
	// Name carries the checked domain name and its availability.
	Name DomainCheckName `xml:"domain:name"`
}

// DomainCheckName is the <domain:name avail="..."> element in a check response.
type DomainCheckName struct {
	// Value is the domain name string.
	Value string `xml:",chardata"`
	// Avail is true if the name is available for registration.
	Avail bool `xml:"avail,attr"`
}

// DomainCheckResponse is parsed from the <domain:chkData> element in <epp:resData>
// for domain:check responses.
type DomainCheckResponse struct {
	XMLName xml.Name `xml:"urn:ietf:params:xml:ns:domain-1.0 domain:chkData"`
	// Results lists each checked domain name with its availability.
	Results []DomainCheckResult `xml:"domain:cd"`
}

// Ensure the domain namespace constant is used to avoid "declared and not used" errors.
var _ = domainNS
