package registrar

import "context"

// Compile-time assertion: stubRegistrar must satisfy the Registrar interface.
// If the Registrar interface gains or changes a method, this line fails at compile
// time — before any test binary is linked.
var _ Registrar = (*stubRegistrar)(nil)

// stubRegistrar is an unexported test double that satisfies Registrar by panicking
// on every method call. It exists solely for the compile-time assertion above.
type stubRegistrar struct{}

func (s *stubRegistrar) Name() string { panic("stub") }

func (s *stubRegistrar) Login(_ context.Context) error  { panic("stub") }
func (s *stubRegistrar) Logout(_ context.Context) error { panic("stub") }
func (s *stubRegistrar) Ping(_ context.Context) error   { panic("stub") }

// DomainChecker satisfies the DomainChecker interface.
func (s *stubRegistrar) CheckDomain(_ context.Context, _ ...string) ([]DomainResult, error) {
	panic("stub")
}

// DomainReader satisfies the DomainReader interface.
func (s *stubRegistrar) InfoDomain(_ context.Context, _ string) (DomainResult, error) {
	panic("stub")
}

// DomainWriter satisfies the DomainWriter interface.
func (s *stubRegistrar) CreateDomain(_ context.Context, _ DomainCreateRequest) (DomainResult, error) {
	panic("stub")
}

func (s *stubRegistrar) UpdateDomain(_ context.Context, _ DomainUpdateRequest) (DomainResult, error) {
	panic("stub")
}

func (s *stubRegistrar) DeleteDomain(_ context.Context, _ string) error { panic("stub") }

func (s *stubRegistrar) RenewDomain(_ context.Context, _ string, _ int) (DomainResult, error) {
	panic("stub")
}

func (s *stubRegistrar) TransferDomain(_ context.Context, _ DomainTransferRequest) (DomainResult, error) {
	panic("stub")
}

// ContactManager satisfies the ContactManager interface.
func (s *stubRegistrar) CheckContact(_ context.Context, _ ...string) ([]ContactResult, error) {
	panic("stub")
}

func (s *stubRegistrar) InfoContact(_ context.Context, _ string) (ContactResult, error) {
	panic("stub")
}

func (s *stubRegistrar) CreateContact(_ context.Context, _ ContactCreateRequest) (ContactResult, error) {
	panic("stub")
}

func (s *stubRegistrar) UpdateContact(_ context.Context, _ ContactUpdateRequest) (ContactResult, error) {
	panic("stub")
}

func (s *stubRegistrar) DeleteContact(_ context.Context, _ string) error { panic("stub") }

// HostManager satisfies the HostManager interface.
func (s *stubRegistrar) CheckHost(_ context.Context, _ ...string) ([]HostResult, error) {
	panic("stub")
}

func (s *stubRegistrar) InfoHost(_ context.Context, _ string) (HostResult, error) {
	panic("stub")
}

func (s *stubRegistrar) CreateHost(_ context.Context, _ HostCreateRequest) (HostResult, error) {
	panic("stub")
}

func (s *stubRegistrar) UpdateHost(_ context.Context, _ HostUpdateRequest) (HostResult, error) {
	panic("stub")
}

func (s *stubRegistrar) DeleteHost(_ context.Context, _ string) error { panic("stub") }

// Poller satisfies the Poller interface.
func (s *stubRegistrar) PollRead(_ context.Context) (PollMessage, error) { panic("stub") }
func (s *stubRegistrar) PollAck(_ context.Context, _ string) error       { panic("stub") }
