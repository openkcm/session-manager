package domain

import (
	"net/url"
	"testing"
)

func TestDomainFromRequest(t *testing.T) {
	// create the test cases
	tests := []struct {
		name string
		url  *url.URL
		want string
	}{
		{
			name: "normal case",
			url: &url.URL{
				Scheme: "https",
				Host:   "example.com",
			},
			want: "example.com",
		}, {
			name: "more complex url",
			url: &url.URL{
				Scheme:   "https",
				Host:     "example.com",
				Opaque:   "/path?query=1",
				Path:     "/foo",
				Fragment: "section1",
			},
			want: "example.com",
		},
	}
	// run the tests
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Arrange

			// Act
			got := domainFromRequest(tc.url)

			// Assert
			if got != tc.want {
				t.Errorf("domainFromRequest() = %v, want %v", got, tc.want)
			}
		})
	}
}
