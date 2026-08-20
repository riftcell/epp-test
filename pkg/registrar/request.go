package registrar

// DomainCreateRequest holds the parameters for a domain:create command.
type DomainCreateRequest struct {
	// Name is the fully qualified domain name to register.
	Name string `mapstructure:"name"`
	// Registrant is the contact ID for the domain registrant.
	Registrant string `mapstructure:"registrant"`
	// Nameservers is the list of nameserver host names.
	Nameservers []string `mapstructure:"nameservers"`
	// AuthInfo is the authorization code for the domain.
	AuthInfo string `mapstructure:"auth_info"`
	// Period is the registration length in years.
	Period int `mapstructure:"period"`
	// Extensions holds registrar-specific extension parameters keyed by namespace URI.
	Extensions map[string]any `mapstructure:"extensions"`
}

// DomainUpdateRequest holds the parameters for a domain:update command.
// Add and Remove slices follow the EPP add/rem structure.
type DomainUpdateRequest struct {
	// Name is the domain to update.
	Name string `mapstructure:"name"`
	// AddNameservers are host names to add to the domain's nameserver delegation.
	AddNameservers []string `mapstructure:"add_nameservers"`
	// RemoveNameservers are host names to remove from the nameserver delegation.
	RemoveNameservers []string `mapstructure:"remove_nameservers"`
	// AddStatuses are EPP status values to add (e.g., "clientHold").
	AddStatuses []string `mapstructure:"add_statuses"`
	// RemoveStatuses are EPP status values to remove.
	RemoveStatuses []string `mapstructure:"remove_statuses"`
	// Extensions holds registrar-specific extension parameters.
	Extensions map[string]any `mapstructure:"extensions"`
}

// DomainTransferRequest holds the parameters for domain:transfer commands.
type DomainTransferRequest struct {
	// Name is the domain subject to the transfer operation.
	Name string `mapstructure:"name"`
	// Op is the transfer operation: "request", "approve", "cancel", "reject", or "query".
	Op string `mapstructure:"op"`
	// AuthInfo is the authorization code, required for "request" operations.
	AuthInfo string `mapstructure:"auth_info"`
	// Extensions holds registrar-specific extension parameters.
	Extensions map[string]any `mapstructure:"extensions"`
}

// ContactCreateRequest holds the parameters for a contact:create command.
type ContactCreateRequest struct {
	// ID is the desired contact handle (e.g., "JDOE-12345"). May be empty for auto-assigned.
	ID string `mapstructure:"id"`
	// Name is the contact's full name.
	Name string `mapstructure:"name"`
	// Org is the organisation name (may be empty).
	Org string `mapstructure:"org"`
	// Email is the contact's email address.
	Email string `mapstructure:"email"`
	// Street, City, StateProvince, PostalCode, CountryCode form the postal address.
	Street        []string `mapstructure:"street"`
	City          string   `mapstructure:"city"`
	StateProvince string   `mapstructure:"state_province"`
	PostalCode    string   `mapstructure:"postal_code"`
	CountryCode   string   `mapstructure:"country_code"`
	// Phone is the voice telephone number in +CC.NNNNNNNNNN format.
	Phone string `mapstructure:"phone"`
	// Fax is the fax number (may be empty).
	Fax string `mapstructure:"fax"`
	// AuthInfo is the contact authorization code.
	AuthInfo string `mapstructure:"auth_info"`
	// Extensions holds registrar-specific extension parameters.
	Extensions map[string]any `mapstructure:"extensions"`
}

// ContactUpdateRequest holds the parameters for a contact:update command.
type ContactUpdateRequest struct {
	// ID is the contact handle to update.
	ID string `mapstructure:"id"`
	// Name replaces the contact name if non-empty.
	Name string `mapstructure:"name"`
	// Org replaces the organisation name if non-empty.
	Org string `mapstructure:"org"`
	// Email replaces the contact email if non-empty.
	Email string `mapstructure:"email"`
	// Extensions holds registrar-specific extension parameters.
	Extensions map[string]any `mapstructure:"extensions"`
}

// HostCreateRequest holds the parameters for a host:create command.
type HostCreateRequest struct {
	// Name is the fully qualified host name to register.
	Name string `mapstructure:"name"`
	// Addrs is the list of IPv4/IPv6 glue addresses for subordinate hosts.
	// For external hosts (not subordinate to a sponsored domain), Addrs must be empty.
	Addrs []string `mapstructure:"addrs"`
}

// HostUpdateRequest holds the parameters for a host:update command.
type HostUpdateRequest struct {
	// Name is the host to update.
	Name string `mapstructure:"name"`
	// AddAddrs are glue addresses to add.
	AddAddrs []string `mapstructure:"add_addrs"`
	// RemoveAddrs are glue addresses to remove.
	RemoveAddrs []string `mapstructure:"remove_addrs"`
	// AddStatuses are EPP status values to add.
	AddStatuses []string `mapstructure:"add_statuses"`
	// RemoveStatuses are EPP status values to remove.
	RemoveStatuses []string `mapstructure:"remove_statuses"`
}
