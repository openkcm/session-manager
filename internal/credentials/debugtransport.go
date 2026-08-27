package credentials

import (
	"net/http"

	"github.com/openkcm/session-manager/internal/debugtools"
)

var debugSettingSMDumpTransport = debugtools.NewSetting("smdumptransport")

func debugTransport(transport http.RoundTripper) http.RoundTripper {
	if debugSettingSMDumpTransport.Value() == "1" {
		return debugtools.NewTransport(transport)
	}

	return transport
}
