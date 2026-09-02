package validation

import (
	"errors"
	"testing"
)

func TestSecureScheme(t *testing.T) {
	tests := []struct {
		name          string
		url           string
		allowInsecure bool
		wantErr       bool
	}{
		{"http rejected when insecure not allowed", "http://example.com", false, true},
		{"http allowed when insecure allowed", "http://example.com", true, false},
		{"https allowed", "https://example.com/introspect", false, false},
		{"https allowed when insecure allowed", "https://example.com", true, false},
		{"non-http scheme allowed", "urn:example:resource", false, false},
		{"empty string treated as no scheme", "", false, false},
		{"unparseable url treated as no scheme", "http://[::1", false, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := SecureScheme(tt.url, tt.allowInsecure)

			if !tt.wantErr {
				if err != nil {
					t.Fatalf("SecureScheme(%q, %v) = %v, want nil", tt.url, tt.allowInsecure, err)
				}
				return
			}

			if err == nil {
				t.Fatalf("SecureScheme(%q, %v) = nil, want error", tt.url, tt.allowInsecure)
			}
			var schemeErr SchemeNotAllowedError
			if !errors.As(err, &schemeErr) {
				t.Fatalf("error = %T, want SchemeNotAllowedError", err)
			}
			if schemeErr.Scheme != schemeHTTP {
				t.Errorf("Scheme = %q, want %q", schemeErr.Scheme, schemeHTTP)
			}
		})
	}
}

func TestSchemeNotAllowedError_Error(t *testing.T) {
	err := SchemeNotAllowedError{Scheme: "http"}
	if got, want := err.Error(), `"http" scheme is not allowed`; got != want {
		t.Errorf("Error() = %q, want %q", got, want)
	}
}
