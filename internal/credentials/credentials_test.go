package credentials

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"strings"
	"testing"
)

// localRoundTripper is an http.RoundTripper that executes HTTP transactions by
// using handler directly, instead of going over an HTTP connection.
type localRoundTripper struct {
	handler http.Handler
}

func (l localRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	w := httptest.NewRecorder()
	l.handler.ServeHTTP(w, req)
	return w.Result(), nil
}

var noContentHandler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusNoContent)
})

var noContentRT = localRoundTripper{
	handler: noContentHandler,
}

func Test_clientAuthRoundTripper_RoundTrip(t *testing.T) {
	ctx := t.Context()
	tests := []struct {
		name         string
		clientID     string
		clientSecret string
		req          *http.Request
		header       http.Header
		next         http.RoundTripper
		wantQuery    url.Values
		wantErr      bool
		wantCt       string
	}{
		{
			name:         "Round trip",
			clientID:     "client-id",
			clientSecret: "secret",
			req:          httptest.NewRequestWithContext(ctx, http.MethodPost, "https://example.com", strings.NewReader(url.Values{}.Encode())),
			header:       http.Header{"Content-Type": []string{"application/x-www-form-urlencoded"}},
			next:         noContentRT,
			wantQuery: url.Values{
				"client_id":     []string{"client-id"},
				"client_secret": []string{"secret"},
			},
			wantCt: urlencoded,
		},
		{
			name:         "No body and no Content-Type",
			clientID:     "client-id",
			clientSecret: "secret",
			req:          httptest.NewRequestWithContext(ctx, http.MethodPost, "https://example.com", nil),
			next:         noContentRT,
			wantQuery: url.Values{
				"client_id":     []string{"client-id"},
				"client_secret": []string{"secret"},
			},
			wantCt: urlencoded,
		},
		{
			name:         "Preserve query values",
			clientID:     "client-id",
			clientSecret: "secret",
			req: httptest.NewRequestWithContext(ctx, http.MethodPost, "https://example.com", strings.NewReader(url.Values{
				"token": []string{"some_token"},
			}.Encode())),
			next: noContentRT,
			wantQuery: url.Values{
				"client_id":     []string{"client-id"},
				"client_secret": []string{"secret"},
				"token":         []string{"some_token"},
			},
			wantCt: urlencoded,
		},
		{
			name:         "Ignore non-post method",
			clientID:     "client-id",
			clientSecret: "secret",
			req:          httptest.NewRequestWithContext(ctx, http.MethodGet, "https://example.com", nil),
			next:         noContentRT,
			wantQuery:    url.Values{},
		},
		{
			name:         "Preserve original Content-Type",
			clientID:     "client-id",
			clientSecret: "secret",
			req:          httptest.NewRequestWithContext(ctx, http.MethodPost, "https://example.com", nil),
			header:       http.Header{contentType: []string{"application/x-www-form-urlencoded; charset=UTF-8"}},
			next:         noContentRT,
			wantQuery: url.Values{
				"client_id":     []string{"client-id"},
				"client_secret": []string{"secret"},
			},
			wantCt: "application/x-www-form-urlencoded; charset=UTF-8",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rt := &clientAuthRoundTripper{
				clientID:     tt.clientID,
				clientSecret: tt.clientSecret,
				next:         tt.next,
			}
			for k, vs := range tt.header {
				tt.req.Header[k] = append(tt.req.Header[k], vs...)
			}
			_, err := rt.RoundTrip(tt.req)
			if (err != nil) != tt.wantErr {
				t.Errorf("clientAuthRoundTripper.RoundTrip() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if ct := tt.req.Header.Get(contentType); ct != tt.wantCt {
				t.Errorf("clientAuthRoundTripper.RoundTrip() contentType = %s, want %s", ct, tt.wantCt)
			}

			b, err := io.ReadAll(tt.req.Body)
			if err != nil {
				t.Fatal("failed to read body", err)
			}

			q, err := url.ParseQuery(string(b))
			if err != nil {
				t.Fatal("failed to parse query", err)
			}

			if len(q) != len(tt.wantQuery) {
				t.Errorf("clientAuthRoundTripper.RoundTrip() query = %s, want %s", q, tt.wantQuery)
			}

			for k, wantVals := range tt.wantQuery {
				gotVals := q[k]
				if !slices.Equal(gotVals, wantVals) {
					t.Errorf("clientAuthRoundTripper.RoundTrip() query[%s] = %s, want %s", k, gotVals, wantVals)
				}
			}
		})
	}
}
