// Package runner executes YAML scenario files against a Registrar.
//
// A scenario file describes an ordered sequence of EPP or RRI operations
// with expected results. The runner drives a registrar.Registrar instance,
// captures response fields for variable interpolation, and handles t.Cleanup
// teardown in reverse dependency order.
//
// See Phase 4 for implementation.
package runner
