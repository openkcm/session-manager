// Package debugtools contains tools used for debugging the application.
package debugtools

import (
	"net/http"
	"net/http/httputil"

	slogctx "github.com/veqryn/slog-context"
)

// transport is a wrapper for an http.RoundTripper which logs the request and response dumps.
type transport struct {
	base http.RoundTripper
}

func NewTransport(base http.RoundTripper) http.RoundTripper {
	return &transport{
		base: base,
	}
}

func (t *transport) RoundTrip(req *http.Request) (*http.Response, error) {
	ctx := req.Context()
	reqDump, _ := httputil.DumpRequestOut(req, true)
	ctx = slogctx.With(ctx, "request", string(reqDump))
	resp, err := t.base.RoundTrip(req)
	if err != nil {
		slogctx.Debug(ctx, "http request executed with an error")
	} else {
		respDump, _ := httputil.DumpResponse(resp, true)
		slogctx.Debug(ctx, "http request executed successfully", "response", string(respDump))
	}

	return resp, err
}
