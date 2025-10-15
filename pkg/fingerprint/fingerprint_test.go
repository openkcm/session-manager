package fingerprint

import (
	"context"
	"net/http"
	"net/http/httptest"
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

func TestFingerprintCtxMiddlewareAndExtractFingerprint(t *testing.T) {
	tests := []struct {
		name           string
		headers        map[string][]string
		expectError    bool
		expectedFPFunc func(*http.Request) (string, error)
	}{
		{
			name:        "no headers",
			headers:     map[string][]string{},
			expectError: false,
			expectedFPFunc: func(r *http.Request) (string, error) {
				return FromHTTPRequest(r)
			},
		},
		{
			name: "with headers",
			headers: map[string][]string{
				"User-Agent": {"Foo"},
				"Accept":     {"Bar"},
			},
			expectError: false,
			expectedFPFunc: func(r *http.Request) (string, error) {
				return FromHTTPRequest(r)
			},
		},
		{
			name: "missing accept header",
			headers: map[string][]string{
				"User-Agent": {"AgentX"},
			},
			expectError: false,
			expectedFPFunc: func(r *http.Request) (string, error) {
				return FromHTTPRequest(r)
			},
		},
		{
			name: "multiple values for headers",
			headers: map[string][]string{
				"User-Agent": {"A", "B"},
				"Accept":     {"C", "D"},
			},
			expectError: false,
			expectedFPFunc: func(r *http.Request) (string, error) {
				return FromHTTPRequest(r)
			},
		},
		{
			name: "case-insensitive header keys",
			headers: map[string][]string{
				"user-agent": {"foo"},
				"ACCEPT":     {"bar"},
			},
			expectError: false,
			expectedFPFunc: func(r *http.Request) (string, error) {
				return FromHTTPRequest(r)
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			for k, vs := range tc.headers {
				for _, v := range vs {
					req.Header.Add(k, v)
				}
			}
			rr := httptest.NewRecorder()

			var gotFP string
			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				fp, err := ExtractFingerprint(r.Context())
				if tc.expectError && err == nil {
					t.Error("expected error, got nil")
				}
				if !tc.expectError && err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				gotFP = fp
			})

			mw := FingerprintCtxMiddleware(handler)
			mw.ServeHTTP(rr, req)

			wantFP, _ := tc.expectedFPFunc(req)
			if gotFP != wantFP {
				t.Errorf("expected fingerprint %s, got %s", wantFP, gotFP)
			}
		})
	}
}

func TestExtractFingerprint_NoFingerprint(t *testing.T) {
	ctx := context.Background()
	_, err := ExtractFingerprint(ctx)
	if err == nil {
		t.Error("expected error when fingerprint is missing in context")
	}
}

func TestExtractFingerprint_ManualContext(t *testing.T) {
	ctx := context.WithValue(context.Background(), fingerprintKey, "manual-fingerprint")
	fp, err := ExtractFingerprint(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if fp != "manual-fingerprint" {
		t.Errorf("expected fingerprint 'manual-fingerprint', got %s", fp)
	}
}
