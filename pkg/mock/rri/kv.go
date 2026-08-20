package rri

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"strings"
)

// ParseKV parses a KV-format RRI message into a field→values map.
//
// Format: "key: value\n" lines terminated by end-of-string (not a blank line —
// the frame boundary from ReadRRIFrame delimitates messages, not blank lines).
// Empty lines are ignored. Keys are lowercased for case-insensitive lookup.
// Duplicate keys accumulate all values in order.
//
// Source: verified against go-rriclient@v1.26.0/pkg/rri/query.go ParseQueryKV.
func ParseKV(msg string) map[string][]string {
	result := make(map[string][]string)
	for _, line := range strings.Split(msg, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		// Split on first colon only — values may contain colons (e.g., error messages).
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.ToLower(strings.TrimSpace(parts[0]))
		val := strings.TrimSpace(parts[1])
		result[key] = append(result[key], val)
	}
	return result
}

// FormatKV serializes a key→values map into RRI KV wire format.
// Each value produces one "key: value\n" line.
// Keys are written as-is (no case transformation).
//
// Used by the RRI mock to format scripted responses.
func FormatKV(fields map[string][]string) string {
	var sb strings.Builder
	for k, vals := range fields {
		for _, v := range vals {
			fmt.Fprintf(&sb, "%s: %s\n", k, v)
		}
	}
	return sb.String()
}

// firstField returns the first value for key from a ParseKV result map,
// or an empty string if the key is absent or has no values.
func firstField(fields map[string][]string, key string) string {
	vals := fields[strings.ToLower(key)]
	if len(vals) == 0 {
		return ""
	}
	return vals[0]
}

// md5Hex returns the lowercase hex MD5 digest of s.
//
// DENIC RRI requires passwords to be transmitted as MD5 hex digests, not plaintext.
// The go-rriclient library does NOT hash internally — the caller (Phase 3 adapter)
// must pre-hash. The RRI mock stores MD5 hashes and compares against incoming
// (already-hashed) password fields (Pitfall 5: MD5 Password Not Pre-Hashed by Caller).
func md5Hex(s string) string {
	h := md5.Sum([]byte(s))
	return hex.EncodeToString(h[:])
}
