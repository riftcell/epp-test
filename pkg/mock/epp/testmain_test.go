//go:build unit

package epp_test

import (
	"testing"

	"go.uber.org/goleak"
)

// TestMain installs goleak goroutine leak detection for the entire epp package
// test suite (MOCK-03, CONVENTIONS.md §5: mandatory in packages that start goroutines).
// If any goroutine from this package leaks after all tests complete, the binary
// exits with a non-zero status.
func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m)
}
