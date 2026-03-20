package credentials

import (
	"net/http"
	"net/http/httptest"
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
	tests := []struct {
		name         string
		clientID     string
		clientSecret string
		req          *http.Request
		next         http.RoundTripper
		wantErr      bool
	}{
		{
			name:         "Round trip",
			clientID:     "client-id",
			clientSecret: "secret",
			req:          httptest.NewRequest(http.MethodGet, "https://example.com", nil),
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
			_, err := rt.RoundTrip(tt.req)
			if (err != nil) != tt.wantErr {
				t.Errorf("clientAuthRoundTripper.RoundTrip() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			q := tt.req.URL.Query()
			clientID := q["client_id"][0]
			clientSecret := q["client_secret"][0]
			if clientID != tt.clientID {
				t.Errorf("clientAuthRoundTripper.RoundTrip() client_id = %s, want %s", clientID, tt.clientID)
			}
			if clientSecret != tt.clientSecret {
				t.Errorf("clientAuthRoundTripper.RoundTrip() client_secret = %s, want %s", clientID, tt.clientSecret)
			}
		})
	}
}
