package xml

import "github.com/nbio/xml"

// EPPEnvelope is the root <epp:epp> element that wraps every EPP frame
// (command, response, or greeting). Exactly one of Command, Response, or
// Greeting is non-nil in any given frame.
type EPPEnvelope struct {
	XMLName  xml.Name     `xml:"urn:ietf:params:xml:ns:epp-1.0 epp:epp"`
	Command  *EPPCommand  `xml:"epp:command,omitempty"`
	Response *EPPResponse `xml:"epp:response,omitempty"`
	Greeting *Greeting    `xml:"epp:greeting,omitempty"`
}

// EPPCommand is the <epp:command> element that contains exactly one command
// body (domain, contact, host, login, logout, or poll). The adapter sets exactly
// one body pointer and leaves the rest nil. ClTRID is the client transaction ID.
type EPPCommand struct {
	// Domain commands
	DomainCheck    *DomainCheckCommand  `xml:",omitempty"`
	DomainCreate   *DomainCreateCommand `xml:",omitempty"`
	DomainInfo     *DomainInfoCommand   `xml:",omitempty"`
	DomainUpdate   *DomainUpdateCommand `xml:",omitempty"`
	DomainDelete   *DomainDeleteCommand `xml:",omitempty"`
	DomainRenew    *DomainRenewCommand  `xml:",omitempty"`
	DomainTransfer *DomainTransfer      `xml:",omitempty"`
	// Contact commands
	ContactCheck  *ContactCheckCommand  `xml:",omitempty"`
	ContactCreate *ContactCreateCommand `xml:",omitempty"`
	ContactInfo   *ContactInfoCommand   `xml:",omitempty"`
	ContactUpdate *ContactUpdateCommand `xml:",omitempty"`
	ContactDelete *ContactDeleteCommand `xml:",omitempty"`
	// Host commands
	HostCheck  *HostCheckCommand  `xml:",omitempty"`
	HostCreate *HostCreateCommand `xml:",omitempty"`
	HostInfo   *HostInfoCommand   `xml:",omitempty"`
	HostUpdate *HostUpdateCommand `xml:",omitempty"`
	HostDelete *HostDeleteCommand `xml:",omitempty"`
	// Session commands
	Login  *Login  `xml:",omitempty"`
	Logout *Logout `xml:",omitempty"`
	Poll   *Poll   `xml:",omitempty"`
	// Extension holds pre-encoded extension XML injected by registrar-specific
	// build hooks. It is placed inside <epp:command> after the core command body.
	Extension *RawExtension `xml:"epp:extension,omitempty"`
	// ClTRID is the client transaction identifier for correlating responses.
	ClTRID string `xml:"epp:clTRID"`
}

// RawExtension wraps raw pre-encoded extension XML. Build hooks write into Inner
// when extending a command; parse hooks read from Inner to extract extension data.
type RawExtension struct {
	Inner []byte `xml:",innerxml"`
}

// EPPResponse is the <epp:response> element returned by the server for every command.
type EPPResponse struct {
	// Result carries the EPP result code and message.
	Result Result `xml:"epp:result"`
	// MsgQ is present on poll responses and carries queue metadata.
	MsgQ *MsgQ `xml:"epp:msgQ,omitempty"`
	// ResData holds object-specific result data (domain:infData, domain:chkData, etc.).
	ResData *RawResData `xml:"epp:resData,omitempty"`
	// Extension holds extension data returned by the server.
	Extension *RawExtension `xml:"epp:extension,omitempty"`
}

// Result is the EPP result element carrying a numeric code and message.
// Code 1000 means success; codes 2xxx indicate errors (RFC 5730 §3).
type Result struct {
	// Code is the EPP result code (e.g., 1000 = success, 2302 = already exists).
	Code int `xml:"code,attr"`
	// Msg is the human-readable result message from the server.
	Msg string `xml:"epp:msg"`
	// Reason is an optional extended error reason, present in some 2xxx responses.
	Reason string `xml:"epp:extValue>epp:reason,omitempty"`
}

// RawResData holds the raw inner XML of the <epp:resData> element so that each
// adapter can unmarshal the object-specific payload (domain:infData, etc.)
// independently using xml.Unmarshal on Inner.
type RawResData struct {
	Inner []byte `xml:",innerxml"`
}

// MsgQ carries poll message queue metadata from the server. Count and ID come
// from attributes on the <epp:msgQ> element; Msg is the human-readable content.
// Maps to PollMessage.QueueDepth, PollMessage.ID, and PollMessage.Content.
type MsgQ struct {
	// Count is the number of messages remaining in the server queue.
	Count int `xml:"count,attr"`
	// ID is the message identifier used to acknowledge (dequeue) this message.
	ID string `xml:"id,attr"`
	// Msg is the human-readable message text, if present.
	Msg string `xml:"epp:msg,omitempty"`
}

// Greeting is the unsolicited <epp:greeting> frame the server sends on every
// new connection. The adapter must read and parse it before sending <epp:login>.
// See RFC 5730 §2.4.
type Greeting struct {
	XMLName xml.Name `xml:"urn:ietf:params:xml:ns:epp-1.0 epp:greeting"`
	// SvID is the server identifier string (used for logging).
	SvID string `xml:"epp:svID"`
	// SvcMenu describes supported EPP versions, languages, object types, and extensions.
	SvcMenu SvcMenu `xml:"epp:svcMenu"`
}

// SvcMenu describes the service capabilities advertised in the EPP greeting.
type SvcMenu struct {
	// Version lists supported EPP versions (typically ["1.0"]).
	Version []string `xml:"epp:version"`
	// Lang lists supported response languages (typically ["en"]).
	Lang []string `xml:"epp:lang"`
	// ObjURI lists supported object namespaces (domain, contact, host URIs).
	ObjURI []string `xml:"epp:objURI"`
	// SvcExtension lists extension namespaces the server supports.
	// The adapter filters login extension requests against this list.
	SvcExtension *SvcExtension `xml:"epp:svcExtension,omitempty"`
}

// SvcExtension holds the list of extension namespace URIs advertised by the server.
type SvcExtension struct {
	// ExtURI lists the extension namespace URIs supported by the server.
	ExtURI []string `xml:"epp:extURI"`
}

// Login is the <epp:login> command that establishes an EPP session.
// The adapter sends this immediately after reading the server greeting.
type Login struct {
	XMLName xml.Name `xml:"urn:ietf:params:xml:ns:epp-1.0 epp:login"`
	// ClID is the registrar client identifier.
	ClID string `xml:"epp:clID"`
	// PW is the registrar password.
	PW string `xml:"epp:pw"`
	// Options specifies the EPP version and response language for the session.
	Options LoginOptions `xml:"epp:options"`
	// Svcs lists the object URIs and extension URIs to enable for the session.
	Svcs LoginSvcs `xml:"epp:svcs"`
}

// LoginOptions carries the EPP protocol version and response language selection.
type LoginOptions struct {
	// Version is the EPP protocol version to use (e.g., "1.0").
	Version string `xml:"epp:version"`
	// Lang is the preferred response language (e.g., "en").
	Lang string `xml:"epp:lang"`
}

// LoginSvcs lists the object type namespaces and extension namespaces requested
// for the session. The adapter intersects these with the server's greeting svcMenu.
type LoginSvcs struct {
	// ObjURI lists the object namespace URIs requested (domain, contact, host).
	ObjURI []string `xml:"epp:objURI"`
	// SvcExtension lists the extension namespace URIs requested for the session.
	SvcExtension *SvcExtension `xml:"epp:svcExtension,omitempty"`
}

// Logout is the <epp:logout> command that terminates an EPP session gracefully.
type Logout struct {
	XMLName xml.Name `xml:"urn:ietf:params:xml:ns:epp-1.0 epp:logout"`
}

// Poll is the <epp:poll> command for retrieving or acknowledging server messages.
// Op must be "req" to retrieve the next message or "ack" to dequeue a specific message.
// MsgID is required when Op is "ack".
type Poll struct {
	XMLName xml.Name `xml:"urn:ietf:params:xml:ns:epp-1.0 epp:poll"`
	// Op is the poll operation: "req" to retrieve, "ack" to acknowledge.
	Op string `xml:"op,attr"`
	// MsgID is the message identifier to acknowledge; required when Op is "ack".
	MsgID string `xml:"msgID,attr,omitempty"`
}
