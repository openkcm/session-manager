package credentials

import (
	"crypto/tls"
	"net/http"
	"reflect"
	"testing"
)

func TestNewTLS(t *testing.T) {
	tests := []struct {
		name      string
		clientID  string
		tlsConfig *tls.Config
		want      *TLS
	}{
		{
			name:      "Success",
			clientID:  "client-id",
			tlsConfig: &tls.Config{},
			want: &TLS{
				ClientID:  "client-id",
				TLSConfig: &tls.Config{},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := NewTLS(tt.clientID, tt.tlsConfig); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("NewTLS() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestTLS_Transport(t *testing.T) {
	tests := []struct {
		name      string
		ClientID  string
		TLSConfig *tls.Config
		want      http.RoundTripper
	}{
		{
			name:      "Success",
			ClientID:  "client-id",
			TLSConfig: &tls.Config{},
			want: &clientAuthRoundTripper{
				clientID: "client-id",
				next: &http.Transport{
					TLSClientConfig: &tls.Config{},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &TLS{
				ClientID:  tt.ClientID,
				TLSConfig: tt.TLSConfig,
			}
			if got := c.Transport(); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("TLS.Transport() = %v, want %v", got, tt.want)
			}
		})
	}
}
