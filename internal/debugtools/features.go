package debugtools

import (
	"net/http"
)

var debugSettingSMDumpTransport = NewSetting("smdumptransport")

// DebugTransport wraps an [http.RoundTripper] with NewTransport if smdebugtransport setting is set.
// Noop is the setting isn't set.
func DebugTransport(rt http.RoundTripper) http.RoundTripper {
	if debugSettingSMDumpTransport.Value() == "1" {
		if _, ok := rt.(*transport); !ok {
			return NewTransport(rt)
		}
	}

	return rt
}
