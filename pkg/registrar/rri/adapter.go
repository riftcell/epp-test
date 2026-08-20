package rri

import (
	"context"
	"fmt"
	"net"
	"strconv"
	"time"

	gorri "github.com/DENICeG/go-rriclient/pkg/rri"

	"github.com/riftcell/epp-test/pkg/config"
	"github.com/riftcell/epp-test/pkg/registrar"
)

// Compile-time assertion: RRIAdapter must satisfy registrar.Registrar.
var _ registrar.Registrar = (*RRIAdapter)(nil)

// RRIAdapter wraps go-rriclient to implement the registrar.Registrar interface
// for DENIC's RRI (Registry Registrar Interface) protocol.
//
// Passwords are pre-hashed with MD5 before Login (DENIC requirement).
// NoAutoRetry is set to true to prevent go-rriclient from internally re-sending
// the already-hashed password on reconnect (which would double-hash it).
type RRIAdapter struct {
	cfg    config.RegistrarConfig
	client *gorri.Client
}

// Option is a functional option for RRIAdapter.
type Option func(*gorri.ClientConfig)

// WithClientConfig overrides the go-rriclient ClientConfig used when dialing.
//
// Used in unit tests to inject a plain-TCP (non-TLS) dialer so the adapter
// can connect to MockRRIServer (which is plain TCP, not TLS):
//
//	WithClientConfig(&rri.ClientConfig{
//	    TLSDialHandler: func(network, addr string, _ *tls.Config) (rri.TLSConnection, error) {
//	        return net.Dial(network, addr)
//	    },
//	})
func WithClientConfig(cfg *gorri.ClientConfig) Option {
	return func(c *gorri.ClientConfig) {
		if cfg != nil {
			*c = *cfg
		}
	}
}

// NewRRIAdapter creates a new RRIAdapter and establishes the TCP/TLS connection
// to the DENIC RRI server. The session is not authenticated until Login is called.
//
// cfg.Host and cfg.Port must be set. An error is returned if the connection
// cannot be established.
func NewRRIAdapter(cfg config.RegistrarConfig, opts ...Option) (*RRIAdapter, error) {
	port := strconv.Itoa(cfg.Port)
	if cfg.Port == 0 {
		port = "51131" // DENIC RRI default port
	}
	addr := net.JoinHostPort(cfg.Host, port)

	// Start with default ClientConfig and apply options.
	clientCfg := &gorri.ClientConfig{}
	for _, opt := range opts {
		opt(clientCfg)
	}

	client, err := gorri.NewClient(addr, clientCfg)
	if err != nil {
		return nil, fmt.Errorf("rri: connect to %s: %w", addr, err)
	}

	// CRITICAL: Disable go-rriclient's internal auto-retry / reconnect.
	//
	// When go-rriclient loses a connection and NoAutoRetry is false (default),
	// it transparently re-dials and re-sends the stored password verbatim.
	// Because we pre-hash the password before calling Login, the stored value is
	// already md5Hex(password). A second hash on reconnect would produce
	// md5Hex(md5Hex(password)), which the DENIC server would reject.
	//
	// Setting NoAutoRetry = true forces callers (or the adapter) to handle
	// reconnects explicitly, ensuring the password is always hashed exactly once.
	//
	// Ref: Pitfall 6 / Open Question 1 in 03-RESEARCH.md.
	client.NoAutoRetry = true

	return &RRIAdapter{
		cfg:    cfg,
		client: client,
	}, nil
}

// Name returns the canonical registrar identifier for DENIC RRI.
func (a *RRIAdapter) Name() string { return "denic" }

// Close closes the underlying TCP connection.
//
// Call Close when you are done with the adapter. In integration tests, use
// t.Cleanup to register Close. Login/Logout do not close the TCP connection —
// Logout only ends the authenticated session; Close tears down the transport.
func (a *RRIAdapter) Close() error {
	return a.client.Close() //nolint:wrapcheck // thin close delegation; error is self-identifying from go-rriclient
}

// Login establishes an authenticated RRI session.
//
// The password from cfg is pre-hashed with MD5 before transmission —
// the wire frame never carries the plaintext password (RRI-04 requirement).
func (a *RRIAdapter) Login(ctx context.Context) error {
	_ = ctx // RRI has no context-aware dial; connection is pre-established.
	if err := a.client.Login(a.cfg.Username, md5Hex(a.cfg.Password)); err != nil {
		return fmt.Errorf("rri: login: %w", err)
	}
	return nil
}

// Logout terminates the RRI session.
func (a *RRIAdapter) Logout(ctx context.Context) error {
	_ = ctx
	if err := a.client.Logout(); err != nil {
		return fmt.Errorf("rri: logout: %w", err)
	}
	return nil
}

// Ping checks liveness of the RRI connection.
//
// go-rriclient has no dedicated ping/noop command. We use QUEUE-READ as a
// lightweight liveness check: even an empty queue returns a success response,
// which proves the connection and session are alive. This matches D-05.
func (a *RRIAdapter) Ping(ctx context.Context) error {
	_ = ctx
	resp, err := a.client.SendQuery(gorri.NewQueueReadQuery(""))
	if err != nil {
		return fmt.Errorf("rri: ping: %w", err)
	}
	// An empty queue (RESULT: success with no messages) is still a successful ping.
	// A failed response would indicate a server-side problem.
	if resp != nil && !resp.IsSuccessful() {
		if err := respToError(resp, "ping"); err != nil {
			return fmt.Errorf("rri: ping: %w", err)
		}
	}
	return nil
}

// CheckDomain checks availability for one or more .de domain names.
//
// Returns one DomainResult per name. Available is set to true when the domain
// is free to register.
func (a *RRIAdapter) CheckDomain(ctx context.Context, names ...string) ([]registrar.DomainResult, error) {
	_ = ctx
	results := make([]registrar.DomainResult, 0, len(names))
	for _, name := range names {
		aceName, err := registrar.NormalizeToACE(name)
		if err != nil {
			return nil, err //nolint:wrapcheck // *EPPError{Code:2005} carries full context
		}
		resp, err := a.client.SendQuery(gorri.NewCheckDomainQuery(aceName))
		if err != nil {
			return nil, fmt.Errorf("rri: check domain %q: %w", aceName, err)
		}
		if err := respToError(resp, "domain:check"); err != nil {
			return nil, err
		}
		avail := parseDomainAvailable(resp)
		results = append(results, registrar.DomainResult{
			Name:      aceName,
			Available: &avail,
		})
	}
	return results, nil
}

// InfoDomain retrieves the full domain object for the given .de name.
func (a *RRIAdapter) InfoDomain(ctx context.Context, name string) (registrar.DomainResult, error) {
	_ = ctx
	aceName, err := registrar.NormalizeToACE(name)
	if err != nil {
		return registrar.DomainResult{}, err //nolint:wrapcheck // *EPPError{Code:2005} carries full context
	}
	name = aceName
	resp, err := a.client.SendQuery(gorri.NewInfoDomainQuery(name))
	if err != nil {
		return registrar.DomainResult{}, fmt.Errorf("rri: info domain %q: %w", name, err)
	}
	if err := respToError(resp, "domain:info"); err != nil {
		return registrar.DomainResult{}, err
	}
	return parseDomainResult(name, resp), nil
}

// CreateDomain registers a new .de domain.
//
// req.Registrant is used as the holder handle. req.Nameservers become the
// domain's nserver entries.
func (a *RRIAdapter) CreateDomain(ctx context.Context, req registrar.DomainCreateRequest) (registrar.DomainResult, error) {
	_ = ctx
	ace, err := registrar.NormalizeToACE(req.Name)
	if err != nil {
		return registrar.DomainResult{}, err //nolint:wrapcheck // *EPPError{Code:2005} carries full context
	}
	req.Name = ace
	data := gorri.DomainData{
		NameServers: req.Nameservers,
	}
	if req.Registrant != "" {
		h, err := gorri.ParseDenicHandle(req.Registrant)
		if err == nil {
			data.HolderHandles = []gorri.DenicHandle{h}
		}
	}
	resp, err := a.client.SendQuery(gorri.NewCreateDomainQuery(req.Name, data))
	if err != nil {
		return registrar.DomainResult{}, fmt.Errorf("rri: create domain %q: %w", req.Name, err)
	}
	if err := respToError(resp, "domain:create"); err != nil {
		return registrar.DomainResult{}, err
	}
	return registrar.DomainResult{Name: req.Name}, nil
}

// UpdateDomain modifies a .de domain's nameservers or holder.
func (a *RRIAdapter) UpdateDomain(ctx context.Context, req registrar.DomainUpdateRequest) (registrar.DomainResult, error) {
	_ = ctx
	ace, err := registrar.NormalizeToACE(req.Name)
	if err != nil {
		return registrar.DomainResult{}, err //nolint:wrapcheck // *EPPError{Code:2005} carries full context
	}
	req.Name = ace
	data := gorri.DomainData{
		NameServers: req.AddNameservers,
	}
	resp, err := a.client.SendQuery(gorri.NewUpdateDomainQuery(req.Name, data))
	if err != nil {
		return registrar.DomainResult{}, fmt.Errorf("rri: update domain %q: %w", req.Name, err)
	}
	if err := respToError(resp, "domain:update"); err != nil {
		return registrar.DomainResult{}, err
	}
	return registrar.DomainResult{Name: req.Name}, nil
}

// DeleteDomain removes a .de domain object.
func (a *RRIAdapter) DeleteDomain(ctx context.Context, name string) error {
	_ = ctx
	aceName, err := registrar.NormalizeToACE(name)
	if err != nil {
		return err //nolint:wrapcheck // *EPPError{Code:2005} carries full context
	}
	name = aceName
	resp, err := a.client.SendQuery(gorri.NewDeleteDomainQuery(name))
	if err != nil {
		return fmt.Errorf("rri: delete domain %q: %w", name, err)
	}
	return respToError(resp, "domain:delete")
}

// RenewDomain is not supported by DENIC RRI.
//
// DENIC handles renewals through billing and auto-renewal — there is no
// explicit renew command in the RRI protocol.
func (a *RRIAdapter) RenewDomain(_ context.Context, _ string, _ int) (registrar.DomainResult, error) {
	return registrar.DomainResult{}, &registrar.EPPError{
		Code:    2101,
		Message: "command not implemented in DENIC RRI",
		Command: "domain:renew",
	}
}

// TransferDomain maps transfer operations to DENIC RRI equivalents.
//
// Operation mapping:
//   - "request" → TRANSIT query (rri.NewTransitDomainQuery, disconnect=false)
//   - "authinfo1" → CREATE-AUTHINFO1 (creates an authorization code with 5-day expiry)
//   - "authinfo2" → CREATE-AUTHINFO2 (redeems the authorization code for the incoming registrar)
//   - all other ops → *EPPError{Code: 2101, command not implemented}
func (a *RRIAdapter) TransferDomain(ctx context.Context, req registrar.DomainTransferRequest) (registrar.DomainResult, error) {
	_ = ctx
	ace, err := registrar.NormalizeToACE(req.Name)
	if err != nil {
		return registrar.DomainResult{}, err //nolint:wrapcheck // *EPPError{Code:2005} carries full context
	}
	req.Name = ace
	switch req.Op {
	case "request":
		// TRANSIT: initiate domain transfer; disconnect=false means the domain stays connected.
		resp, err := a.client.SendQuery(gorri.NewTransitDomainQuery(req.Name, false))
		if err != nil {
			return registrar.DomainResult{}, fmt.Errorf("rri: transit domain %q: %w", req.Name, err)
		}
		if err := respToError(resp, "domain:transfer/transit"); err != nil {
			return registrar.DomainResult{}, err
		}
		return registrar.DomainResult{Name: req.Name}, nil

	case "authinfo1":
		// CREATE-AUTHINFO1: generate an auth code valid for 5 days.
		expiry := time.Now().Add(5 * 24 * time.Hour)
		resp, err := a.client.SendQuery(gorri.NewCreateAuthInfo1Query(req.Name, req.AuthInfo, expiry))
		if err != nil {
			return registrar.DomainResult{}, fmt.Errorf("rri: create-authinfo1 %q: %w", req.Name, err)
		}
		if err := respToError(resp, "domain:transfer/authinfo1"); err != nil {
			return registrar.DomainResult{}, err
		}
		return registrar.DomainResult{Name: req.Name}, nil

	case "authinfo2":
		// CREATE-AUTHINFO2: incoming registrar redeems the auth code.
		resp, err := a.client.SendQuery(gorri.NewCreateAuthInfo2Query(req.Name))
		if err != nil {
			return registrar.DomainResult{}, fmt.Errorf("rri: create-authinfo2 %q: %w", req.Name, err)
		}
		if err := respToError(resp, "domain:transfer/authinfo2"); err != nil {
			return registrar.DomainResult{}, err
		}
		return registrar.DomainResult{Name: req.Name}, nil

	default:
		return registrar.DomainResult{}, &registrar.EPPError{
			Code:    2101,
			Message: fmt.Sprintf("transfer op %q not implemented in DENIC RRI", req.Op),
			Command: "domain:transfer",
		}
	}
}

// CheckContact checks whether one or more contact handles exist in DENIC RRI.
func (a *RRIAdapter) CheckContact(ctx context.Context, ids ...string) ([]registrar.ContactResult, error) {
	_ = ctx
	results := make([]registrar.ContactResult, 0, len(ids))
	for _, id := range ids {
		h, err := gorri.ParseDenicHandle(id)
		if err != nil {
			return nil, fmt.Errorf("rri: check contact: invalid handle %q: %w", id, err)
		}
		resp, err := a.client.SendQuery(gorri.NewCheckHandleQuery(h))
		if err != nil {
			return nil, fmt.Errorf("rri: check contact %q: %w", id, err)
		}
		avail := !resp.IsSuccessful() // handle does not exist → available
		results = append(results, registrar.ContactResult{
			ID: id,
			// Available is not a field on ContactResult; success means the handle exists.
			// We encode existence in the Status slice for compatibility.
			Status: func() []string {
				if resp.IsSuccessful() {
					return []string{"found"}
				}
				return []string{"not-found"}
			}(),
			Extensions: map[string]any{"available": avail},
		})
	}
	return results, nil
}

// InfoContact retrieves the full contact handle object.
func (a *RRIAdapter) InfoContact(ctx context.Context, id string) (registrar.ContactResult, error) {
	_ = ctx
	h, err := gorri.ParseDenicHandle(id)
	if err != nil {
		return registrar.ContactResult{}, fmt.Errorf("rri: info contact: invalid handle %q: %w", id, err)
	}
	resp, err := a.client.SendQuery(gorri.NewInfoHandleQuery(h))
	if err != nil {
		return registrar.ContactResult{}, fmt.Errorf("rri: info contact %q: %w", id, err)
	}
	if err := respToError(resp, "contact:info"); err != nil {
		return registrar.ContactResult{}, err
	}
	return parseContactResult(id, resp), nil
}

// CreateContact registers a new DENIC contact handle.
func (a *RRIAdapter) CreateContact(ctx context.Context, req registrar.ContactCreateRequest) (registrar.ContactResult, error) {
	_ = ctx
	h, err := gorri.ParseDenicHandle(req.ID)
	if err != nil {
		return registrar.ContactResult{}, fmt.Errorf("rri: create contact: invalid handle %q: %w", req.ID, err)
	}
	data := gorri.ContactData{
		Type:         gorri.ContactTypePerson,
		Name:         req.Name,
		Organisation: req.Org,
		Address:      joinStrings(req.Street),
		PostalCode:   req.PostalCode,
		City:         req.City,
		CountryCode:  req.CountryCode,
		EMail:        []string{req.Email},
		Phone:        req.Phone,
	}
	resp, err := a.client.SendQuery(gorri.NewCreateContactQuery(h, data))
	if err != nil {
		return registrar.ContactResult{}, fmt.Errorf("rri: create contact %q: %w", req.ID, err)
	}
	if err := respToError(resp, "contact:create"); err != nil {
		return registrar.ContactResult{}, err
	}
	return registrar.ContactResult{ID: req.ID, Name: req.Name, Email: req.Email}, nil
}

// UpdateContact is not directly supported by RRI as a standalone update command.
// DENIC RRI uses CHHOLDER (change-holder) for contact updates on domains.
// For handle-level updates, DENIC requires contacting support.
// This method returns EPPError{Code: 2101}.
func (a *RRIAdapter) UpdateContact(_ context.Context, _ registrar.ContactUpdateRequest) (registrar.ContactResult, error) {
	return registrar.ContactResult{}, &registrar.EPPError{
		Code:    2101,
		Message: "contact update not supported in DENIC RRI — use CHHOLDER for domain holder changes",
		Command: "contact:update",
	}
}

// DeleteContact is not supported in DENIC RRI.
func (a *RRIAdapter) DeleteContact(_ context.Context, _ string) error {
	return &registrar.EPPError{
		Code:    2101,
		Message: "contact delete not supported in DENIC RRI",
		Command: "contact:delete",
	}
}

// CheckHost returns EPPError{Code: 2101} — DENIC RRI has no host objects.
func (a *RRIAdapter) CheckHost(_ context.Context, _ ...string) ([]registrar.HostResult, error) {
	return nil, &registrar.EPPError{
		Code:    2101,
		Message: "host operations not supported by DENIC RRI",
		Command: "host:check",
	}
}

// InfoHost returns EPPError{Code: 2101} — DENIC RRI has no host objects.
func (a *RRIAdapter) InfoHost(_ context.Context, _ string) (registrar.HostResult, error) {
	return registrar.HostResult{}, &registrar.EPPError{
		Code:    2101,
		Message: "host operations not supported by DENIC RRI",
		Command: "host:info",
	}
}

// CreateHost returns EPPError{Code: 2101} — DENIC RRI has no host objects.
func (a *RRIAdapter) CreateHost(_ context.Context, _ registrar.HostCreateRequest) (registrar.HostResult, error) {
	return registrar.HostResult{}, &registrar.EPPError{
		Code:    2101,
		Message: "host operations not supported by DENIC RRI",
		Command: "host:create",
	}
}

// UpdateHost returns EPPError{Code: 2101} — DENIC RRI has no host objects.
func (a *RRIAdapter) UpdateHost(_ context.Context, _ registrar.HostUpdateRequest) (registrar.HostResult, error) {
	return registrar.HostResult{}, &registrar.EPPError{
		Code:    2101,
		Message: "host operations not supported by DENIC RRI",
		Command: "host:update",
	}
}

// DeleteHost returns EPPError{Code: 2101} — DENIC RRI has no host objects.
func (a *RRIAdapter) DeleteHost(_ context.Context, _ string) error {
	return &registrar.EPPError{
		Code:    2101,
		Message: "host operations not supported by DENIC RRI",
		Command: "host:delete",
	}
}

// PollRead retrieves the next pending message from the DENIC message queue.
//
// Returns EPPError{Code: 1300} when the queue is empty (matching the Poller contract).
func (a *RRIAdapter) PollRead(ctx context.Context) (registrar.PollMessage, error) {
	_ = ctx
	resp, err := a.client.SendQuery(gorri.NewQueueReadQuery(""))
	if err != nil {
		return registrar.PollMessage{}, fmt.Errorf("rri: poll read: %w", err)
	}
	if !resp.IsSuccessful() {
		// An empty queue is indicated by a failure result in RRI.
		// Map to EPP 1300 (queue empty) per the Poller contract.
		errMsgs := resp.ErrorMessages()
		if len(errMsgs) > 0 {
			// Check if this is a "queue empty" scenario by message ID.
			// DENIC uses business code in the error — treat any poll failure
			// as a potential empty-queue indication.
			msg := errMsgs[0]
			// Business code 83000000006 indicates "no messages in queue" (DENIC-specific).
			// We map all poll-read failures to EPP 1300 (no messages) for interface
			// compatibility, as callers expect this code for empty-queue detection.
			return registrar.PollMessage{}, &registrar.EPPError{
				Code:    1300,
				Message: msg.Message(),
				Command: "poll:read",
			}
		}
		return registrar.PollMessage{}, &registrar.EPPError{
			Code:    1300,
			Message: "no messages in queue",
			Command: "poll:read",
		}
	}

	// Parse the poll message from the response fields.
	msgID := resp.FirstField("MSGID")
	msgType := resp.FirstField("MSGTYPE")
	info := resp.FirstField("INFO")
	return registrar.PollMessage{
		ID:      msgID,
		Type:    msgType,
		Content: info,
	}, nil
}

// PollAck acknowledges and removes a message from the DENIC message queue.
func (a *RRIAdapter) PollAck(ctx context.Context, msgID string) error {
	_ = ctx
	resp, err := a.client.SendQuery(gorri.NewQueueDeleteQuery(msgID, ""))
	if err != nil {
		return fmt.Errorf("rri: poll ack %q: %w", msgID, err)
	}
	return respToError(resp, "poll:ack")
}

// respToError converts a failed RRI response to a *registrar.EPPError.
//
// If resp is nil or successful, returns nil.
//
// RRI business codes are mapped to EPP-style codes as follows:
//   - 83xxxxxxxx series → 2400 (command failed, generic)
//   - No specific mapping table is maintained; the raw business message text is
//     preserved in EPPError.Message for diagnostic purposes.
func respToError(resp *gorri.Response, command string) error {
	if resp == nil || resp.IsSuccessful() {
		return nil
	}
	errMsgs := resp.ErrorMessages()
	if len(errMsgs) == 0 {
		return &registrar.EPPError{
			Code:    2400,
			Message: "RRI command failed (no error details)",
			Command: command,
		}
	}
	msg := errMsgs[0]
	// Map RRI business codes to EPP result codes.
	// RRI uses numeric IDs in the 83xxxxxxxx range; EPP uses 1xxx/2xxx.
	// We default to 2400 (command failed) for unrecognized codes.
	eppCode := mapRRICodeToEPP(msg.ID())
	return &registrar.EPPError{
		Code:    eppCode,
		Message: msg.Message(),
		Command: command,
	}
}

// mapRRICodeToEPP maps a DENIC RRI business message ID to an EPP result code.
//
// DENIC business codes are in the 83xxxxxxxxxx range. This function provides
// a best-effort mapping; unmapped codes fall through to 2400 (command failed).
func mapRRICodeToEPP(rriCode int64) int {
	// Known DENIC business codes (partial list from DENIC documentation):
	//   83000000001 — object not found → EPP 2303
	//   83000000002 — object already exists → EPP 2302
	//   83000000010 — authentication error → EPP 2201
	switch rriCode {
	case 83000000001:
		return 2303 // object does not exist
	case 83000000002:
		return 2302 // object exists
	case 83000000010:
		return 2201 // authorization error
	default:
		return 2400 // command failed (generic)
	}
}

// parseDomainAvailable reads the check response to determine domain availability.
//
// DENIC RRI CHECK returns RESULT: success for an available domain and
// RESULT: failure (with a "domain already exists" error) for a taken domain.
func parseDomainAvailable(resp *gorri.Response) bool {
	return resp.IsSuccessful()
}

// parseDomainResult maps RRI INFO domain response fields to DomainResult.
func parseDomainResult(name string, resp *gorri.Response) registrar.DomainResult {
	result := registrar.DomainResult{Name: name}

	// Holder handle.
	if holder := resp.FirstField("HOLDER"); holder != "" {
		result.Registrant = holder
	}

	// Status is not directly available in RRI INFO; set a synthetic "ok" for success.
	result.Status = []string{"ok"}

	return result
}

// parseContactResult maps RRI INFO handle response fields to ContactResult.
func parseContactResult(id string, resp *gorri.Response) registrar.ContactResult {
	result := registrar.ContactResult{ID: id}
	result.Name = resp.FirstField("NAME")
	result.Org = resp.FirstField("ORGANISATION")
	emails := resp.Field("EMAIL")
	if len(emails) > 0 {
		result.Email = emails[0]
	}
	return result
}

// joinStrings joins a slice of strings with newlines (for RRI multi-line address fields).
func joinStrings(ss []string) string {
	result := ""
	for i, s := range ss {
		if i > 0 {
			result += "\n"
		}
		result += s
	}
	return result
}
