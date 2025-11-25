package config

import "net/http"

func (ct *CookieTemplate) ToCookie(value string) *http.Cookie {
	var sameSite http.SameSite
	switch ct.SameSite {
	case CookieSameSiteNone:
		sameSite = http.SameSiteNoneMode
	case CookieSameSiteLax:
		sameSite = http.SameSiteLaxMode
	case CookieSameSiteStrict:
		sameSite = http.SameSiteStrictMode
	}

	return &http.Cookie{
		Name:     ct.Name,
		Value:    value,
		MaxAge:   ct.MaxAge,
		Path:     ct.Path,
		Domain:   ct.Domain,
		Secure:   ct.Secure,
		HttpOnly: ct.HTTPOnly,
		SameSite: sameSite,
	}
}
