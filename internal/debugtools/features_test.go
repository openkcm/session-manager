package debugtools

import (
	"net/http"
	"testing"
)

func TestDebugTransport_Enabled(t *testing.T) {
	// Enable the smdumptransport feature for this test and restore it after.
	parse("smdumptransport=1")
	t.Cleanup(func() { parse("smdumptransport=") })

	var base http.RoundTripper = http.DefaultTransport
	got := DebugTransport(base)
	if _, ok := got.(*transport); !ok {
		t.Errorf("DebugTransport() = %T, want *transport when smdumptransport=1", got)
	}
}

func TestDebugTransport_Disabled(t *testing.T) {
	parse("smdumptransport=")
	var base http.RoundTripper = http.DefaultTransport
	got := DebugTransport(base)
	if got != base {
		t.Errorf("DebugTransport() = %T, want original transport when disabled", got)
	}
}

func TestDebugTransport_AlreadyWrapped(t *testing.T) {
	parse("smdumptransport=1")
	t.Cleanup(func() { parse("smdumptransport=") })

	// Wrapping an already-wrapped transport should not double-wrap.
	inner := NewTransport(http.DefaultTransport)
	got := DebugTransport(inner)
	if got != inner {
		t.Errorf("DebugTransport() re-wrapped a *transport, want identity")
	}
}
