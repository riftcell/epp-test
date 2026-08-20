//go:build unit

package epp_test

import (
	"fmt"

	"github.com/riftcell/epp-test/pkg/mock/epp"
)

// exampleStub is a minimal testing.TB stub for use in Example functions.
// It panics on Fatal (surfacing failures as panics) and accumulates Cleanup
// functions to be run explicitly at the end of the Example.
type exampleStub struct {
	cleanups []func()
}

func (s *exampleStub) Helper()           {}
func (s *exampleStub) Fatal(args ...any) { panic(fmt.Sprint(args...)) }
func (s *exampleStub) Cleanup(f func())  { s.cleanups = append(s.cleanups, f) }
func (s *exampleStub) runCleanup() {
	for i := len(s.cleanups) - 1; i >= 0; i-- {
		s.cleanups[i]()
	}
}

// ExampleMockEPPServer demonstrates creating an in-process EPP mock server.
// In a real test function pass *testing.T instead of exampleStub.
func ExampleMockEPPServer() {
	tb := &exampleStub{}
	defer tb.runCleanup()

	srv := epp.NewMockEPPServer(tb)
	fmt.Println(srv.Addr() != "")
	// Output: true
}
