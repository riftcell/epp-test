package rri

import (
	"crypto/md5"
	"encoding/hex"
)

// md5Hex returns the lowercase hex MD5 digest of s.
//
// DENIC RRI requires passwords to be transmitted as MD5 hex digests, not plaintext.
// go-rriclient does NOT hash internally — the adapter pre-hashes before calling
// client.Login so that the wire frame never carries the plaintext password.
//
// This function is a deliberate copy of the unexported md5Hex in the mock/rri package.
// Adapters must not import mock packages, and the mock's function is unexported,
// so each package that needs it owns its own copy. The implementation is trivial
// (5 lines) and the MD5 algorithm is stable — divergence risk is negligible.
func md5Hex(s string) string {
	h := md5.Sum([]byte(s))
	return hex.EncodeToString(h[:])
}
