//go:build unit

package runner

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/riftcell/epp-test/pkg/registrar"
)

func TestInterpolateStringExact(t *testing.T) {
	captured := map[string]map[string]any{
		"create": {"id": "C-1"},
	}
	got := interpolateString("${create.id}", captured)
	require.Equal(t, "C-1", got)
}

func TestInterpolateStringSubstring(t *testing.T) {
	captured := map[string]map[string]any{
		"create": {"id": "C-1"},
	}
	got := interpolateString("prefix-${create.id}-suffix", captured)
	require.Equal(t, "prefix-C-1-suffix", got)
}

func TestInterpolateStringNoRefs(t *testing.T) {
	captured := map[string]map[string]any{}
	got := interpolateString("no refs here", captured)
	require.Equal(t, "no refs here", got)
}

func TestInterpolateStringMissingField(t *testing.T) {
	// Missing step/field must remain as the literal token, not become "".
	captured := map[string]map[string]any{}
	got := interpolateString("${missing.field}", captured)
	require.Equal(t, "${missing.field}", got, "missing reference must remain as literal token, not silently empty")
	require.NotEmpty(t, got, "missing reference must not become empty string")
}

func TestCaptureResult(t *testing.T) {
	contact := registrar.ContactResult{
		ID:    "C-1",
		Email: "a@b.c",
	}
	m := captureResult(contact)
	require.Equal(t, "C-1", m["ID"], "ID field must be captured")
	require.Equal(t, "a@b.c", m["Email"], "Email field must be captured")
}

func TestRunID(t *testing.T) {
	// When RUN_ID env var is set, use it as-is.
	t.Setenv("RUN_ID", "fixed-id")
	got := runID()
	require.Equal(t, "fixed-id", got)

	// When RUN_ID is unset, generate an 8-hex-char random string.
	t.Setenv("RUN_ID", "")
	got2 := runID()
	require.NotEmpty(t, got2, "generated run-ID must be non-empty")
	require.Len(t, got2, 8, "generated run-ID must be 8 hex characters")
	// Verify it is valid hex: only 0-9 and a-f.
	for _, c := range got2 {
		require.True(t, (c >= '0' && c <= '9') || (c >= 'a' && c <= 'f'),
			"generated run-ID must contain only hex characters, got: %c", c)
	}
}

func TestInjectRunIDPrefix(t *testing.T) {
	prefix := "abc12345"

	t.Run("create_domain prefixes name", func(t *testing.T) {
		params := map[string]any{"name": "example.at", "period": 1}
		out := injectRunIDPrefix("create_domain", params, prefix)
		require.Equal(t, prefix+"-example.at", out["name"])
		require.Equal(t, 1, out["period"], "non-name fields must be unchanged")
	})

	t.Run("create_contact prefixes id", func(t *testing.T) {
		params := map[string]any{"id": "C-001", "email": "a@b.c"}
		out := injectRunIDPrefix("create_contact", params, prefix)
		require.Equal(t, prefix+"-C-001", out["id"])
		require.Equal(t, "a@b.c", out["email"])
	})

	t.Run("variable reference skipped", func(t *testing.T) {
		params := map[string]any{"name": "${check_contact.id}"}
		out := injectRunIDPrefix("create_domain", params, prefix)
		require.Equal(t, "${check_contact.id}", out["name"],
			"variable references must not have prefix injected")
	})

	t.Run("non-create op untouched", func(t *testing.T) {
		params := map[string]any{"name": "example.at"}
		out := injectRunIDPrefix("delete_domain", params, prefix)
		require.Equal(t, "example.at", out["name"],
			"non-create ops must not have prefix injected")
	})
}
