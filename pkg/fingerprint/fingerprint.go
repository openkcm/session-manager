package fingerprint

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"

	envoyauth "github.com/envoyproxy/go-control-plane/envoy/service/auth/v3"
)

var headerKeys = []string{"user-agent", "accept"}

func FromHTTPRequest(r *http.Request) (string, error) {
	if r == nil {
		return "", errors.New("http request is nil")
	}

	h := sha256.New()

	for _, key := range headerKeys {
		h.Write([]byte(r.Header.Get(key)))
	}

	return hex.EncodeToString(h.Sum(nil)), nil
}

func FromEnvoyHTTPRequest(r *envoyauth.AttributeContext_HttpRequest) (string, error) {
	if r == nil {
		return "", errors.New("envoy http request is nil")
	}

	h := sha256.New()

	for _, key := range headerKeys {
		if v, ok := r.GetHeaders()[key]; ok {
			h.Write([]byte(v))
		} else {
			h.Write([]byte(""))
		}
	}

	return hex.EncodeToString(h.Sum(nil)), nil
}
