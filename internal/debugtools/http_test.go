package debugtools

import (
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
	ctx := t.Context()
	const url = "http://localhost"
	resp := httptest.NewRecorder().Result()
	tests := []struct {
		name    string
		base    http.RoundTripper
		req     *http.Request
		want    *http.Response
		wantErr bool
	}{
		{
			name:    "Round trip",
			base:    dummyRoundTripper{resp: resp, err: nil},
			req:     httptest.NewRequestWithContext(ctx, http.MethodGet, url, nil),
			want:    resp,
			wantErr: false,
		},
		{
			name:    "Return an error",
			base:    dummyRoundTripper{resp: nil, err: errors.New("err")},
			req:     httptest.NewRequestWithContext(ctx, http.MethodGet, url, nil),
			want:    nil,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tr := NewTransport(tt.base)
			got, err := tr.RoundTrip(tt.req)
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
