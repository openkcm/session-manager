package credentials

import (
	"net/http"
	"reflect"
	"testing"
)

func TestNewInsecure(t *testing.T) {
	tests := []struct {
		name string
		want TransportCredentials
	}{
		{
			name: "New insecure",
			want: &Insecure{
				clientID: "client-id",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := NewInsecure("client-id"); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("NewInsecure() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestInsecure_Transport(t *testing.T) {
	tests := []struct {
		name string
		c    *Insecure
		want http.RoundTripper
	}{
		{
			name: "Transport",
			c:    &Insecure{clientID: "client-id"},
			want: &clientAuthRoundTripper{
				clientID: "client-id",
				next:     http.DefaultTransport,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.c.Transport(); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Insecure.Transport() = %v, want %v", got, tt.want)
			}
		})
	}
}
