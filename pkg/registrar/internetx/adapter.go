package internetx

import (
	"fmt"

	"github.com/riftcell/epp-test/pkg/config"
	"github.com/riftcell/epp-test/pkg/registrar"
	epp "github.com/riftcell/epp-test/pkg/registrar/epp"
)

// Compile-time assertion: InternetXAdapter must implement registrar.Registrar.
// Embedding GenericEPPAdapter promotes all Registrar methods, so this compiles
// with no additional method code.
var _ registrar.Registrar = (*InternetXAdapter)(nil)

// InternetXAdapter is the InternetX EPP registrar adapter (REG-02).
//
// It wraps GenericEPPAdapter with all extension hooks left nil. OT&E discovery
// will later populate hooks without changing this API (D-08).
// The adapter name "internetx" flows to Name() via the embedded engine.
type InternetXAdapter struct {
	*epp.GenericEPPAdapter
}

// NewInternetXAdapter creates an InternetX EPP adapter.
//
// All extension hooks remain nil (no extensions discovered yet per D-08/REG-02).
// Options are passed through to GenericEPPAdapter — unit tests use WithTLSConfig
// to inject the mock server's CA pool and client certificate.
func NewInternetXAdapter(cfg config.RegistrarConfig, opts ...epp.Option) (*InternetXAdapter, error) {
	g, err := epp.NewGenericEPPAdapter("internetx", cfg, opts...)
	if err != nil {
		return nil, fmt.Errorf("internetx adapter: %w", err)
	}
	// No extension hooks discovered yet (REG-02 / D-08); all hooks remain nil.
	return &InternetXAdapter{g}, nil
}
