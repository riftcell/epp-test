package registrar

import (
	"net"
	"time"
)

// DomainResult holds the response fields for domain check and info operations.
// Available is non-nil only for check responses; ROID and timestamps are non-zero
// only for info responses.
type DomainResult struct {
	// Name is the fully qualified domain name.
	Name string
	// Available is non-nil for check responses. True means the name is available.
	Available *bool
	// Status lists the EPP status codes on the object (e.g., "ok", "clientHold").
	Status []string
	// Registrant is the contact ID of the domain registrant.
	Registrant string
	// ROID is the registry object identifier assigned by the registry.
	ROID string
	// CreatedAt is the time the domain was created.
	CreatedAt time.Time
	// ExpiresAt is the time the current registration period expires.
	ExpiresAt time.Time
	// Extensions holds registrar-specific extension data keyed by namespace URI.
	Extensions map[string]any
}

// ContactResult holds the response fields for contact check and info operations.
type ContactResult struct {
	// ID is the contact handle (e.g., "JDOE-12345").
	ID string
	// Name is the contact's full name.
	Name string
	// Org is the organisation name (may be empty).
	Org string
	// Email is the contact's email address.
	Email string
	// Status lists the EPP status codes on the contact object.
	Status []string
	// Extensions holds registrar-specific extension data.
	Extensions map[string]any
}

// HostResult holds the response fields for host check and info operations.
type HostResult struct {
	// Name is the fully qualified host name (nameserver).
	Name string
	// Addrs holds IPv4 and IPv6 glue addresses for subordinate hosts.
	Addrs []net.IP
	// Status lists the EPP status codes on the host object.
	Status []string
	// Extensions holds registrar-specific extension data.
	Extensions map[string]any
}

// PollMessage represents a single message from the EPP server message queue.
type PollMessage struct {
	// ID is the server-assigned message ID used for PollAck.
	ID string
	// Type describes the message category (e.g., "domain transfer", "domain delete").
	Type string
	// Content is the human-readable message body from the server.
	Content string
	// QueueDepth is the number of messages remaining in the queue after this one.
	QueueDepth int
}
