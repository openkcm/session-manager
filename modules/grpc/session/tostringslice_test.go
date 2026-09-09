package session_test

import (
	"testing"

	session "github.com/openkcm/session-manager/modules/grpc/session"
)

func TestToStringSlice(t *testing.T) {
	t.Run("nil returns nil", func(t *testing.T) {
		if session.ToStringSlice(nil) != nil {
			t.Error("ToStringSlice(nil) should return nil")
		}
	})

	t.Run("[]string returns []string", func(t *testing.T) {
		in := []string{"a", "b"}
		got := session.ToStringSlice(in)
		if len(got) != 2 || got[0] != "a" || got[1] != "b" {
			t.Errorf("ToStringSlice(%v) = %v, want same slice", in, got)
		}
	})

	t.Run("[]any of strings returns []string", func(t *testing.T) {
		in := []any{"x", "y"}
		got := session.ToStringSlice(in)
		if len(got) != 2 || got[0] != "x" || got[1] != "y" {
			t.Errorf("ToStringSlice(%v) = %v, want [x y]", in, got)
		}
	})

	t.Run("[]any with non-string element returns nil", func(t *testing.T) {
		in := []any{"ok", 42}
		got := session.ToStringSlice(in)
		if got != nil {
			t.Errorf("ToStringSlice(%v) = %v, want nil when element is not a string", in, got)
		}
	})

	t.Run("unknown type returns nil", func(t *testing.T) {
		if session.ToStringSlice(42) != nil {
			t.Error("ToStringSlice(int) should return nil")
		}
	})
}
