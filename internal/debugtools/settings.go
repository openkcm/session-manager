package debugtools

import (
	"os"
	"sync"
	"sync/atomic"
)

var cache sync.Map // name string -> value *setting
var empty string

type Setting struct {
	*setting

	name string
	once sync.Once
}

type setting struct {
	value atomic.Pointer[string]
}

func NewSetting(name string) *Setting {
	return &Setting{name: name}
}

func (s *Setting) Name() string {
	return s.name
}

func (s *Setting) Value() string {
	s.once.Do(func() {
		s.setting = lookup(s.Name())
	})

	return *s.value.Load()
}

func (s *Setting) String() string {
	return s.Name() + "=" + s.Value()
}

func lookup(name string) *setting {
	if v, ok := cache.Load(name); ok {
		//nolint:forcetypeassert
		return v.(*setting)
	}

	s := new(setting)
	s.value.Store(&empty)
	if v, loaded := cache.LoadOrStore(name, s); loaded {
		//nolint:forcetypeassert
		return v.(*setting)
	}

	return s
}

func init() {
	env := os.Getenv("DEBUGFEATURES")
	parse(env)
}

// parse parses DEBUGFEATURES environment variable in the form of k1=v1,k2=v2,k3=v3
// and stores the keys.
func parse(s string) {
	end := len(s)
	eq := -1
	for i := end - 1; i >= -1; i-- {
		if i == -1 || s[i] == ',' {
			if eq >= 0 {
				name, arg := s[i+1:eq], s[eq+1:end]
				lookup(name).value.Store(&arg)
			}
			eq = -1
			end = i
		} else if s[i] == '=' {
			eq = i
		}
	}
}
