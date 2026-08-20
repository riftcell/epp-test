// Package frame implements RFC 5734 EPP-over-TCP frame framing
// (4-byte big-endian total-length prefix that INCLUDES the header).
// Shared by the EPP mock server (pkg/mock/epp) and the EPP protocol
// adapter (pkg/registrar/epp).
package frame
