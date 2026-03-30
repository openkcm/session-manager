package credentials

import (
	"net/http"
	"reflect"
	"testing"
)

func TestNewClientSecretPost(t *testing.T) {
	tests := []struct {
		name         string
		clientID     string
		clientSecret string
		want         *ClientSecretPost
	}{
		{
			name:         "Success",
			clientID:     "client-id",
			clientSecret: "secret",
			want: &ClientSecretPost{
				ClientID:     "client-id",
				ClientSecret: "secret",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := NewClientSecretPost(tt.clientID, tt.clientSecret); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("NewClientSecretPost() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestClientSecretPost_Transport(t *testing.T) {
	tests := []struct {
		name         string
		ClientID     string
		ClientSecret string
		want         http.RoundTripper
	}{
		{
			name:         "Success",
			ClientID:     "client-id",
			ClientSecret: "secret",
			want: &clientAuthRoundTripper{
				clientID:     "client-id",
				clientSecret: "secret",
				next:         http.DefaultTransport,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &ClientSecretPost{
				ClientID:     tt.ClientID,
				ClientSecret: tt.ClientSecret,
			}
			if got := c.Transport(); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ClientSecret.Transport() = %v, want %v", got, tt.want)
			}
		})
	}
}
