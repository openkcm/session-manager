package credentials

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
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

func Test_clientAuthRoundTripper_RoundTrip(t *testing.T) {
	ctx := t.Context()
	tests := []struct {
		name         string
		clientID     string
		clientSecret string
		req          *http.Request
		header       http.Header
		next         http.RoundTripper
		wantErr      bool
	}{
		{
			name:         "Round trip",
			clientID:     "client-id",
			clientSecret: "secret",
			req:          httptest.NewRequestWithContext(ctx, http.MethodPost, "https://example.com", strings.NewReader(url.Values{}.Encode())),
			header:       http.Header{"Content-Type": []string{"application/x-www-form-urlencoded"}},
			next: localRoundTripper{
				handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusNoContent)
				}),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rt := &clientAuthRoundTripper{
				clientID:     tt.clientID,
				clientSecret: tt.clientSecret,
				next:         tt.next,
			}
			tt.req.Header = tt.header
			_, err := rt.RoundTrip(tt.req)
			if (err != nil) != tt.wantErr {
				t.Errorf("clientAuthRoundTripper.RoundTrip() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			b, err := io.ReadAll(tt.req.Body)
			if err != nil {
				t.Fatal("failed to read body", err)
			}

			q, err := url.ParseQuery(string(b))
			if err != nil {
				t.Fatal("failed to parse query", err)
			}

			clientID := q.Get("client_id")
			clientSecret := q.Get("client_secret")
			if clientID != tt.clientID {
				t.Errorf("clientAuthRoundTripper.RoundTrip() client_id = %s, want %s", clientID, tt.clientID)
			}
			if clientSecret != tt.clientSecret {
				t.Errorf("clientAuthRoundTripper.RoundTrip() client_secret = %s, want %s", clientID, tt.clientSecret)
			}
		})
	}
}
