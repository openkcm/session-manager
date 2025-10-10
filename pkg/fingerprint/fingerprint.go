package fingerprint

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"

	envoyauth "github.com/envoyproxy/go-control-plane/envoy/service/auth/v3"
)

var headerKeys = []string{"user-agent", "accept"}

type ctxKey string

const fingerprintKey ctxKey = "fingerprint"

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

func FingerprintCtxMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fp, _ := FromHTTPRequest(r)
		ctxWithFP := context.WithValue(r.Context(), fingerprintKey, fp)
		next.ServeHTTP(w, r.WithContext(ctxWithFP))
	})
}

func ExtractFingerprint(ctx context.Context) (string, error) {
	fp, ok := ctx.Value(fingerprintKey).(string)
	if !ok {
		return "", errors.New("no fingerprint in ctx")
	}
	return fp, nil
}
