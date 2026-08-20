package denic

import (
	"fmt"

	"github.com/riftcell/epp-test/pkg/config"
	"github.com/riftcell/epp-test/pkg/registrar"
	rri "github.com/riftcell/epp-test/pkg/registrar/rri"
)

// Compile-time assertion: DENICAdapter must implement registrar.Registrar.
// Embedding RRIAdapter promotes all Registrar methods, so this compiles
// with no additional method code.
var _ registrar.Registrar = (*DENICAdapter)(nil)

// DENICAdapter is the DENIC registrar adapter (REG-05).
//
// It is a thin wrapper over RRIAdapter. Name() is inherited from RRIAdapter
// and returns "denic" — no override needed. DENIC-specific field mapping beyond
// the RRI adapter is deferred; for Phase 3 the thin embed is the deliverable (D-09).
type DENICAdapter struct {
	*rri.RRIAdapter
}

// NewDENICAdapter creates a DENIC RRI adapter.
//
// Options are passed through to NewRRIAdapter — unit tests use WithClientConfig
// to inject a plain-TCP dialer so the adapter can connect to MockRRIServer
// (which does not use TLS).
func NewDENICAdapter(cfg config.RegistrarConfig, opts ...rri.Option) (*DENICAdapter, error) {
	r, err := rri.NewRRIAdapter(cfg, opts...)
	if err != nil {
		return nil, fmt.Errorf("denic adapter: %w", err)
	}
	return &DENICAdapter{r}, nil
}
