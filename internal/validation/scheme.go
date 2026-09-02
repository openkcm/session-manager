package validation

import (
	"fmt"
	"net/url"
)

const schemeHTTP = "http"

type SchemeNotAllowedError struct {
	Scheme string
}

func (e SchemeNotAllowedError) Error() string {
	return fmt.Sprintf("%q scheme is not allowed", e.Scheme)
}

func SecureScheme(u string, allowInsecure bool) error {
	if s := scheme(u); s == schemeHTTP && !allowInsecure {
		return SchemeNotAllowedError{Scheme: s}
	}

	return nil
}

func scheme(u string) string {
	if parsed, err := url.Parse(u); err == nil {
		return parsed.Scheme
	}

	return ""
}
