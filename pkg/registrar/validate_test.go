//go:build unit

package registrar_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/riftcell/epp-test/pkg/registrar"
)

func TestValidateDomainName(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"empty passes", "", false},
		{"valid simple", "example.com", false},
		{"valid subdomain", "sub.example.co.uk", false},
		{"valid xn-- ACE label", "xn--mnchen-3ya.de", false},
		{"valid xn-- uppercase prefix", "XN--MNCHEN-3YA.de", false},
		{"label exactly 63 chars", strings.Repeat("a", 63) + ".com", false},
		{"label 64 chars triggers error", strings.Repeat("a", 64) + ".com", true},
		{"leading hyphen", "-bad.com", true},
		{"trailing hyphen", "bad-.com", true},
		{"total length 255 chars", strings.Repeat("a", 63) + "." + strings.Repeat("b", 63) + "." + strings.Repeat("c", 63) + "." + strings.Repeat("d", 61), false},
		{"total length 256 chars", strings.Repeat("a", 256), true},
		{"underscore not allowed", "bad_underscore.com", true},
		{"space not allowed", "bad domain.com", true},
		{"raw unicode münchen.de rejected", "münchen.de", true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := registrar.ValidateDomainName(tc.input)
			if tc.wantErr {
				assert.Error(t, err)
				var eppErr *registrar.EPPError
				assert.True(t, errors.As(err, &eppErr), "expected *EPPError, got %T: %v", err, err)
				if eppErr != nil {
					assert.Equal(t, 2005, eppErr.Code)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateContactHandle(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"empty passes", "", false},
		{"valid alphanumeric-hyphen", "ABC-123", false},
		{"valid single char", "A", false},
		{"valid 16 chars", strings.Repeat("A", 16), false},
		{"valid 64 chars", strings.Repeat("A", 64), false},
		{"65 chars too long", strings.Repeat("A", 65), true},
		{"space not allowed", "bad handle", true},
		{"underscore not allowed", "bad_handle", true},
		{"dot not allowed", "bad.handle", true},
		{"unicode handle Müller rejected", "Müller", true},
		{"unicode handle Jürgen rejected", "Jürgen-1", true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := registrar.ValidateContactHandle(tc.input)
			if tc.wantErr {
				assert.Error(t, err)
				var eppErr *registrar.EPPError
				assert.True(t, errors.As(err, &eppErr), "expected *EPPError, got %T: %v", err, err)
				if eppErr != nil {
					assert.Equal(t, 2005, eppErr.Code)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateAuthInfo(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"empty passes", "", false},
		{"valid 6 chars minimum", "Secret", false},
		{"valid 8 chars", "Secret12", false},
		{"valid 64 chars maximum", strings.Repeat("A", 64), false},
		{"5 chars below minimum", strings.Repeat("A", 5), true},
		{"65 chars above maximum", strings.Repeat("A", 65), true},
		{"non-printable byte 0x1F", "Valid\x1Fstr", true},
		{"non-printable NUL 0x00", "Valid\x00str", true},
		{"non-printable DEL 0x7F", "Valid\x7Fstr", true},
		{"printable ASCII 0x20 space", "Valid string!", false},
		{"printable ASCII 0x7E tilde", "Valid~string", false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := registrar.ValidateAuthInfo(tc.input)
			if tc.wantErr {
				assert.Error(t, err)
				var eppErr *registrar.EPPError
				assert.True(t, errors.As(err, &eppErr), "expected *EPPError, got %T: %v", err, err)
				if eppErr != nil {
					assert.Equal(t, 2005, eppErr.Code)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestNormalizeToACE(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{"unicode to ACE", "münchen.de", "xn--mnchen-3ya.de", false},
		{"pure ASCII passes through", "example.com", "example.com", false},
		{"existing ACE passes through", "xn--mnchen-3ya.de", "xn--mnchen-3ya.de", false},
		{"empty passes", "", "", false},
		{"invalid rejected", "abc-.de", "", true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := registrar.NormalizeToACE(tc.input)
			if tc.wantErr {
				assert.Error(t, err)
				var eppErr *registrar.EPPError
				assert.True(t, errors.As(err, &eppErr), "expected *EPPError, got %T: %v", err, err)
				if eppErr != nil {
					assert.Equal(t, 2005, eppErr.Code)
				}
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tc.want, got)
			}
		})
	}
}

func TestValidateHostName(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"empty passes", "", false},
		{"valid host", "ns1.example.com", false},
		{"valid xn-- label", "xn--mnchen-3ya.de", false},
		{"label too long", strings.Repeat("a", 64) + ".com", true},
		{"leading hyphen", "-ns.example.com", true},
		{"trailing hyphen", "ns-.example.com", true},
		{"underscore not allowed", "ns_1.example.com", true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := registrar.ValidateHostName(tc.input)
			if tc.wantErr {
				assert.Error(t, err)
				var eppErr *registrar.EPPError
				assert.True(t, errors.As(err, &eppErr), "expected *EPPError, got %T: %v", err, err)
				if eppErr != nil {
					assert.Equal(t, 2005, eppErr.Code)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
