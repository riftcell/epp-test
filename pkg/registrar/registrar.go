// Package registrar defines the Registrar interface and its sub-interfaces
// that all EPP and RRI adapters must implement. This is the shared contract
// between mock servers (Phase 2), protocol adapters (Phase 3), and the
// scenario runner (Phase 4).
//
// Interface hierarchy:
//
//	Registrar
//	├── DomainManager  (DomainChecker + DomainReader + DomainWriter)
//	├── ContactManager
//	├── HostManager
//	└── Poller
//
// All methods accept context.Context as the first argument for
// timeout and cancellation support on every network operation.
package registrar

import "context"

// DomainChecker checks domain availability.
type DomainChecker interface {
	// CheckDomain reports availability for one or more domain names.
	// Returns one DomainResult per name; DomainResult.Available is non-nil.
	CheckDomain(ctx context.Context, names ...string) ([]DomainResult, error)
}

// DomainReader retrieves domain objects.
type DomainReader interface {
	// InfoDomain returns the full domain object for the given name.
	InfoDomain(ctx context.Context, name string) (DomainResult, error)
}

// DomainWriter creates and modifies domain objects.
type DomainWriter interface {
	// CreateDomain registers a new domain.
	CreateDomain(ctx context.Context, req DomainCreateRequest) (DomainResult, error)
	// UpdateDomain adds or removes nameservers, contacts, and statuses.
	UpdateDomain(ctx context.Context, req DomainUpdateRequest) (DomainResult, error)
	// DeleteDomain removes a domain object.
	DeleteDomain(ctx context.Context, name string) error
	// RenewDomain extends the registration period by years.
	RenewDomain(ctx context.Context, name string, years int) (DomainResult, error)
	// TransferDomain initiates, approves, cancels, rejects, or queries a transfer.
	TransferDomain(ctx context.Context, req DomainTransferRequest) (DomainResult, error)
}

// DomainManager composes domain read, write, and check operations.
type DomainManager interface {
	DomainChecker
	DomainReader
	DomainWriter
}

// ContactManager manages EPP contact objects.
type ContactManager interface {
	// CheckContact reports whether one or more contact IDs are available.
	CheckContact(ctx context.Context, ids ...string) ([]ContactResult, error)
	// InfoContact returns the full contact object for the given ID.
	InfoContact(ctx context.Context, id string) (ContactResult, error)
	// CreateContact registers a new contact.
	CreateContact(ctx context.Context, req ContactCreateRequest) (ContactResult, error)
	// UpdateContact modifies an existing contact.
	UpdateContact(ctx context.Context, req ContactUpdateRequest) (ContactResult, error)
	// DeleteContact removes a contact object.
	DeleteContact(ctx context.Context, id string) error
}

// HostManager manages EPP host objects (nameservers).
type HostManager interface {
	// CheckHost reports whether one or more host names are available.
	CheckHost(ctx context.Context, names ...string) ([]HostResult, error)
	// InfoHost returns the full host object for the given name.
	InfoHost(ctx context.Context, name string) (HostResult, error)
	// CreateHost registers a new host object with optional glue addresses.
	CreateHost(ctx context.Context, req HostCreateRequest) (HostResult, error)
	// UpdateHost modifies an existing host (e.g., adds/removes glue records).
	UpdateHost(ctx context.Context, req HostUpdateRequest) (HostResult, error)
	// DeleteHost removes a host object.
	DeleteHost(ctx context.Context, name string) error
}

// Poller retrieves and acknowledges server messages from the EPP message queue.
type Poller interface {
	// PollRead retrieves the next pending server message.
	// Returns an error wrapping EPPError{Code: 1300} when the queue is empty.
	PollRead(ctx context.Context) (PollMessage, error)
	// PollAck acknowledges and dequeues the message with the given ID.
	PollAck(ctx context.Context, msgID string) error
}

// Registrar is the aggregate interface passed to test suites and scenario runners.
// All EPP adapters and the DENIC RRI adapter must satisfy this interface.
//
// Implementations are created by the adapter constructors in pkg/registrar/epp/
// and pkg/registrar/rri/ (Phase 3). Tests use the stub in registrar_test.go.
type Registrar interface {
	DomainManager
	ContactManager
	HostManager
	Poller

	// Login establishes a session with the registrar, negotiating service extensions.
	Login(ctx context.Context) error
	// Logout terminates the session.
	Logout(ctx context.Context) error
	// Ping sends a keepalive (EPP <hello>, RRI no-op) and verifies the connection.
	Ping(ctx context.Context) error
	// Name returns the canonical registrar identifier: "internetx", "nicat", "eurid", or "denic".
	Name() string
}
