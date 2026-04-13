package debugtools

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
)

type dummyRoundTripper struct {
	resp *http.Response
	err  error
}

func (rt dummyRoundTripper) RoundTrip(*http.Request) (*http.Response, error) {
	return rt.resp, rt.err
}

func Test_transport_RoundTrip(t *testing.T) {
	const url = "http://localhost"
	resp := httptest.NewRecorder().Result()
	tests := []struct {
		name    string
		base    http.RoundTripper
		want    *http.Response
		wantErr bool
	}{
		{
			name:    "Round trip",
			base:    dummyRoundTripper{resp: resp, err: nil},
			want:    resp,
			wantErr: false,
		},
		{
			name:    "Return an error",
			base:    dummyRoundTripper{resp: nil, err: errors.New("err")},
			want:    nil,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, url, nil)
			if err != nil {
				t.Fatalf("failed to create request: %v", err)
			}
			tr := NewTransport(tt.base)
			got, err := tr.RoundTrip(req)
			if (err != nil) != tt.wantErr {
				t.Errorf("transport.RoundTrip() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("transport.RoundTrip() = %v, want %v", got, tt.want)
			}
		})
	}
}
