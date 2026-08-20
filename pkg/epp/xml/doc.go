// Package xml holds EPP wire-format structs for RFC 5730-5734 commands and responses.
// It uses github.com/nbio/xml (NOT encoding/xml) so marshaled output preserves namespace
// prefixes such as domain:create; Go's stdlib marshaler emits ns0:create, which EPP
// registrars reject (issue #48821).
package xml
