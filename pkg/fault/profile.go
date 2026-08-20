package fault

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

// PerOpRule matches a raw frame substring and applies one or more faults.
// First matching rule in the PerOp list wins (D-02).
type PerOpRule struct {
	// Match is checked via strings.Contains against the full raw frame string.
	Match string `yaml:"match"`
	// Delay is an additional response delay stacked on top of ResponseDelay (additive per D-06).
	// Must be a valid Go duration string (e.g., "500ms"). Empty means no extra delay.
	Delay string `yaml:"delay"`
	// ResultCode overrides the EPP result code. Reserved for future use; 0 means no override.
	ResultCode int `yaml:"result_code"`
	// Mismatch triggers ActionMismatch response for this operation (EPP only, D-08).
	Mismatch bool `yaml:"mismatch"`
	// Disconnect causes ActionDisconnect when this operation matches (D-04 on-op).
	Disconnect bool `yaml:"disconnect"`

	delay time.Duration // resolved from Delay by ParseDurations; unexported
}

// FaultProfile holds the static configuration for fault simulation.
// Treat it as read-only after ParseDurations() is called.
// Duration fields are strings because yaml.v3 cannot parse "2s" into time.Duration.
type FaultProfile struct {
	// ConnectDelay: sleep before any protocol action (D-05). E.g. "2s", "500ms".
	ConnectDelay string `yaml:"connect_delay"`
	// ResponseDelay: global sleep before every response write (D-06). E.g. "100ms".
	ResponseDelay string `yaml:"response_delay"`
	// LoginMode: always|flap|hang|disconnect (D-03). Empty = normal login.
	LoginMode string `yaml:"login_mode"`
	// LoginFailCount: number of initial failures in flap mode (D-03).
	LoginFailCount int `yaml:"login_fail_count"`
	// DisconnectAt: "pre-greeting" or "post-greeting" (D-04). Configured via YAML only.
	DisconnectAt string `yaml:"disconnect_at"`
	// DisconnectAfter: close after N non-login operations. 0 = disabled (D-04).
	DisconnectAfter int `yaml:"disconnect_after"`
	// MalformedFrame: length_overflow|invalid_xml|garbage|"" (D-07). EPP only.
	MalformedFrame string `yaml:"malformed_frame"`
	// FaultMismatch: global op substring enabling mismatch (D-08). E.g. "domain:create".
	FaultMismatch string `yaml:"fault_mismatch"`
	// PerOp: per-operation fault rules. First match wins (D-02).
	PerOp []PerOpRule `yaml:"per_op"`

	connectDelay  time.Duration // resolved by ParseDurations
	responseDelay time.Duration // resolved by ParseDurations
}

// ParseDurations parses all duration string fields into unexported time.Duration fields.
// Must be called after yaml.Unmarshal and after any CLI flag overrides, before NewFaultEngine.
// Returns an error wrapping time.ParseDuration if any string is invalid.
func (p *FaultProfile) ParseDurations() error {
	if p.ConnectDelay != "" {
		d, err := time.ParseDuration(p.ConnectDelay)
		if err != nil {
			return fmt.Errorf("connect_delay: %w", err)
		}
		p.connectDelay = d
	}
	if p.ResponseDelay != "" {
		d, err := time.ParseDuration(p.ResponseDelay)
		if err != nil {
			return fmt.Errorf("response_delay: %w", err)
		}
		p.responseDelay = d
	}
	for i := range p.PerOp {
		if p.PerOp[i].Delay != "" {
			d, err := time.ParseDuration(p.PerOp[i].Delay)
			if err != nil {
				return fmt.Errorf("per_op[%d].delay: %w", i, err)
			}
			p.PerOp[i].delay = d
		}
	}
	return nil
}

// LoadProfile reads and unmarshals a YAML fault profile from path.
// Does NOT call ParseDurations — the caller applies flag overrides then calls ParseDurations.
// Returns a zero-value FaultProfile (no faults) if path is empty.
func LoadProfile(path string) (FaultProfile, error) {
	if path == "" {
		return FaultProfile{}, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return FaultProfile{}, fmt.Errorf("fault: read %q: %w", path, err)
	}
	var p FaultProfile
	if err := yaml.Unmarshal(data, &p); err != nil {
		return FaultProfile{}, fmt.Errorf("fault: parse %q: %w", path, err)
	}
	return p, nil
}
