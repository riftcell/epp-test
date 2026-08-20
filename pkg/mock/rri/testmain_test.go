//go:build unit

package rri_test

import (
	"testing"

	"go.uber.org/goleak"
)

// TestMain installs goleak goroutine leak detection for the rri package
// test suite (MOCK-03, CONVENTIONS.md §5).
func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m)
}
