package xml

import "github.com/nbio/xml"

// HostCheckCommand is the EPP host:check wire frame for checking nameserver availability.
type HostCheckCommand struct {
	XMLName xml.Name `xml:"urn:ietf:params:xml:ns:host-1.0 host:check"`
	// Names lists the fully qualified host names to check.
	Names []string `xml:"host:name"`
}

// HostCreateCommand is the EPP host:create wire frame for registering a nameserver.
type HostCreateCommand struct {
	XMLName xml.Name `xml:"urn:ietf:params:xml:ns:host-1.0 host:create"`
	// Name is the fully qualified host name to register.
	Name string `xml:"host:name"`
	// Addrs lists the IPv4 and IPv6 glue addresses for subordinate hosts.
	// For external (non-subordinate) hosts, this must be empty.
	Addrs []HostAddr `xml:"host:addr,omitempty"`
}

// HostAddr is an IP address element in a host command, carrying the address family.
type HostAddr struct {
	// IP is the IP address string (IPv4 or IPv6 dotted notation).
	IP string `xml:",chardata"`
	// IPVersion is the address family: "v4" for IPv4, "v6" for IPv6.
	IPVersion string `xml:"ip,attr,omitempty"`
}

// HostInfoCommand is the EPP host:info wire frame for retrieving a nameserver object.
type HostInfoCommand struct {
	XMLName xml.Name `xml:"urn:ietf:params:xml:ns:host-1.0 host:info"`
	// Name is the host to retrieve.
	Name string `xml:"host:name"`
}

// HostUpdateCommand is the EPP host:update wire frame for modifying a nameserver.
type HostUpdateCommand struct {
	XMLName xml.Name `xml:"urn:ietf:params:xml:ns:host-1.0 host:update"`
	// Name is the host to update.
	Name string `xml:"host:name"`
	// Add specifies addresses and statuses to add.
	Add *HostUpdateAddRem `xml:"host:add,omitempty"`
	// Rem specifies addresses and statuses to remove.
	Rem *HostUpdateAddRem `xml:"host:rem,omitempty"`
}

// HostUpdateAddRem holds addresses and statuses to add or remove in a host update.
type HostUpdateAddRem struct {
	// Addrs are IP addresses to add or remove.
	Addrs []HostAddr `xml:"host:addr,omitempty"`
	// Statuses are EPP status values to add or remove.
	Statuses []HostStatus `xml:"host:status,omitempty"`
}

// HostStatus represents an EPP status element with an s attribute.
type HostStatus struct {
	// S is the status value string (e.g., "ok", "clientDeleteProhibited").
	S string `xml:"s,attr"`
}

// HostDeleteCommand is the EPP host:delete wire frame for removing a nameserver.
type HostDeleteCommand struct {
	XMLName xml.Name `xml:"urn:ietf:params:xml:ns:host-1.0 host:delete"`
	// Name is the fully qualified host name to delete.
	Name string `xml:"host:name"`
}

// HostInfoResponse is parsed from the <host:infData> element in <epp:resData>
// for host:info responses.
type HostInfoResponse struct {
	XMLName xml.Name `xml:"urn:ietf:params:xml:ns:host-1.0 host:infData"`
	// Name is the fully qualified host name.
	Name string `xml:"host:name"`
	// Addrs lists the glue addresses registered for this host.
	Addrs []HostAddr `xml:"host:addr"`
	// Statuses lists the EPP status codes on the host object.
	Statuses []HostStatus `xml:"host:status"`
}
