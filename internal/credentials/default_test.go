package credentials

import (
	"net/http"
	"reflect"
	"testing"
)

func TestNewDefault(t *testing.T) {
	tests := []struct {
		name string
		want TransportCredentials
	}{
		{
			name: "New default",
			want: &Default{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := NewDefault(); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("NewDefault() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDefault_Transport(t *testing.T) {
	tests := []struct {
		name string
		c    *Default
		want http.RoundTripper
	}{
		{
			name: "Transport",
			c:    &Default{},
			want: http.DefaultTransport,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &Default{}
			if got := c.Transport(); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Default.Transport() = %v, want %v", got, tt.want)
			}
		})
	}
}
