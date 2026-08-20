//go:build unit

package rri

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestParseKVBasic verifies basic key: value parsing.
func TestParseKVBasic(t *testing.T) {
	msg := "version: 5.0\naction: LOGIN\nuser: TEST\n"
	m := ParseKV(msg)
	assert.Equal(t, []string{"5.0"}, m["version"])
	assert.Equal(t, []string{"LOGIN"}, m["action"])
	assert.Equal(t, []string{"TEST"}, m["user"])
}

// TestParseKVKeysLowercased verifies that incoming keys are lowercased.
func TestParseKVKeysLowercased(t *testing.T) {
	msg := "ACTION: LOGIN\nUSER: test\n"
	m := ParseKV(msg)
	assert.Equal(t, []string{"LOGIN"}, m["action"], "key 'ACTION' must become 'action'")
	assert.Equal(t, []string{"test"}, m["user"], "key 'USER' must become 'user'")
	assert.Empty(t, m["ACTION"], "original case key must not be present")
}

// TestParseKVIgnoresBlankLines verifies blank lines do not produce empty entries.
func TestParseKVIgnoresBlankLines(t *testing.T) {
	msg := "key1: val1\n\n\nkey2: val2\n"
	m := ParseKV(msg)
	assert.Equal(t, []string{"val1"}, m["key1"])
	assert.Equal(t, []string{"val2"}, m["key2"])
	assert.Len(t, m, 2, "blank lines must not create entries")
}

// TestParseKVDuplicateKeys verifies multiple values accumulate under the same key.
func TestParseKVDuplicateKeys(t *testing.T) {
	msg := "ns: ns1.example.com\nns: ns2.example.com\nns: ns3.example.com\n"
	m := ParseKV(msg)
	require.Len(t, m["ns"], 3)
	assert.Equal(t, "ns1.example.com", m["ns"][0])
	assert.Equal(t, "ns2.example.com", m["ns"][1])
	assert.Equal(t, "ns3.example.com", m["ns"][2])
}

// TestParseKVValueWithColon verifies that only the first colon is the separator.
func TestParseKVValueWithColon(t *testing.T) {
	msg := "error: 83000000010 Please login first\n"
	m := ParseKV(msg)
	assert.Equal(t, []string{"83000000010 Please login first"}, m["error"])
}

// TestFormatKVRoundTrip verifies that FormatKV output is parseable by ParseKV.
func TestFormatKVRoundTrip(t *testing.T) {
	original := map[string][]string{
		"result": {"success"},
		"stid":   {"abc123"},
	}
	formatted := FormatKV(original)
	assert.NotEmpty(t, formatted)

	parsed := ParseKV(formatted)
	// Values should survive the round-trip (keys lowercased in ParseKV)
	for k, vals := range original {
		assert.Equal(t, vals, parsed[k], "key %q did not round-trip", k)
	}
}

// TestMD5HexKnownVector verifies md5Hex against a known test vector.
// echo -n "secret" | md5sum = 5ebe2294ecd0e0f08eab7690d2a6ee69
// This confirms the mock stores and compares the right hash format (Pitfall 5).
func TestMD5HexKnownVector(t *testing.T) {
	assert.Equal(t, "5ebe2294ecd0e0f08eab7690d2a6ee69", md5Hex("secret"))
	assert.Equal(t, "d41d8cd98f00b204e9800998ecf8427e", md5Hex(""), "MD5 of empty string")
}
