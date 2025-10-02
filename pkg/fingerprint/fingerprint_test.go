package fingerprint

import (
	"net/http"
	"testing"

	envoy "github.com/envoyproxy/go-control-plane/envoy/service/auth/v3"
)

func TestAll(t *testing.T) {
	// create the test cases
	tests := []struct {
		name       string
		req1       *http.Request
		wantError1 bool
		req2       *envoy.AttributeContext_HttpRequest
		wantError2 bool
	}{
		{
			name:       "zero values",
			wantError1: true,
			wantError2: true,
		}, {
			name:       "empty requests",
			req1:       &http.Request{Header: http.Header{}},
			wantError1: false,
			req2:       &envoy.AttributeContext_HttpRequest{Headers: map[string]string{}},
			wantError2: false,
		}, {
			name: "normal requests",
			req1: &http.Request{Header: http.Header{
				"User-Agent": []string{"Foo"},
				"Accept":     []string{"Bar"},
			}},
			wantError1: false,
			req2: &envoy.AttributeContext_HttpRequest{Headers: map[string]string{
				"user-agent": "Foo",
				"accept":     "Bar",
			}},
			wantError2: false,
		},
	}

	// run the tests
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Act 1
			h1, err1 := FromHTTPRequest(tc.req1)

			// Assert 1
			if tc.wantError1 {
				if err1 == nil {
					t.Error("expected error, but got nil")
				}
			} else {
				if err1 != nil {
					t.Errorf("unexpected error: %s", err1)
				}
			}

			// Act 2
			h2, err2 := FromEnvoyHTTPRequest(tc.req2)

			// Assert 2
			if tc.wantError2 {
				if err2 == nil {
					t.Error("expected error, but got nil")
				}
			} else {
				if err2 != nil {
					t.Errorf("unexpected error: %s", err2)
				}
			}

			// Compare the results
			if tc.wantError1 == tc.wantError2 && h1 != h2 {
				t.Errorf("fingerprints do not match: %s != %s", h1, h2)
			}
		})
	}
}
