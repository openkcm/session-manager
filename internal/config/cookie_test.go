package config

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestToCookie(t *testing.T) {
	tests := []struct {
		name     string
		template CookieTemplate
		value    string
		want     *http.Cookie
	}{
		{
			name: "defaults",
			template: CookieTemplate{
				Name: "foo",
			},
			want: &http.Cookie{
				Name:     "foo",
				MaxAge:   0,
				Path:     "",
				Domain:   "",
				Secure:   false,
				SameSite: 0,
				HttpOnly: false,
			},
		}, {
			name: "session",
			template: CookieTemplate{
				Name:     "session",
				Path:     "/",
				Secure:   true,
				SameSite: CookieSameSiteLax,
				HTTPOnly: true,
			},
			want: &http.Cookie{
				Name:     "session",
				MaxAge:   0,
				Path:     "/",
				Domain:   "",
				Secure:   true,
				SameSite: http.SameSiteLaxMode,
				HttpOnly: true,
			},
		}, {
			name: "csrf",
			template: CookieTemplate{
				Name:     "csrf",
				Path:     "/",
				Secure:   true,
				SameSite: CookieSameSiteStrict,
			},
			want: &http.Cookie{
				Name:     "csrf",
				MaxAge:   0,
				Path:     "/",
				Domain:   "",
				Secure:   true,
				SameSite: http.SameSiteStrictMode,
				HttpOnly: false,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := tt.template.ToCookie(tt.value)
			t.Logf("Got cookie: %+v", c)
			assert.Equal(t, tt.want.Name, c.Name)
			assert.Equal(t, tt.want.MaxAge, c.MaxAge)
			assert.Equal(t, tt.want.Path, c.Path)
			assert.Equal(t, tt.want.Domain, c.Domain)
			assert.Equal(t, tt.want.Secure, c.Secure)
			assert.Equal(t, tt.want.SameSite, c.SameSite)
			assert.Equal(t, tt.want.HttpOnly, c.HttpOnly)
		})
	}
}
