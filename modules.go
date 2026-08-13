package sessionmanager

import (
	"fmt"
	"iter"
	"maps"
	"reflect"
	"sync"
)

var (
	modules   = make(map[string]ModuleInfo)
	modulesMu sync.RWMutex
)

func RegisterModule(module Module) {
	modulesMu.Lock()
	defer modulesMu.Unlock()

	info := module.Module()

	if _, ok := modules[info.ID]; ok {
		panic(`module "` + info.ID + `" has already been registered`)
	}

	modules[info.ID] = info
}

func GetModule(id string) (ModuleInfo, error) {
	modulesMu.RLock()
	defer modulesMu.RUnlock()
	mod, ok := modules[id]
	if !ok {
		return ModuleInfo{}, fmt.Errorf("module %q is not registered", id)
	}

	return mod, nil
}

func Modules() iter.Seq[ModuleInfo] {
	modulesMu.RLock()
	defer modulesMu.RUnlock()

	return maps.Values(maps.Clone(modules))
}

// depInterfaces maps a dependency-interface key (`dep` struct tag)
// to the reflect.Type of the interface a target module must implement.
var (
	depInterfaces   = make(map[string]reflect.Type)
	depInterfacesMu sync.RWMutex
)

// RegisterDepInterface associates a dep tag key with the interface type that a
// dependency named by that key must implement. Call it from a package init().
// Duplicate keys panic, mirroring RegisterModule.
func RegisterDepInterface(key string, t reflect.Type) {
	if t == nil || t.Kind() != reflect.Interface {
		panic(`dep interface for "` + key + `" must be a non-nil interface type`)
	}

	depInterfacesMu.Lock()
	defer depInterfacesMu.Unlock()

	if _, ok := depInterfaces[key]; ok {
		panic(`dep interface "` + key + `" has already been registered`)
	}

	depInterfaces[key] = t
}

func lookupDepInterface(key string) (reflect.Type, bool) {
	depInterfacesMu.RLock()
	defer depInterfacesMu.RUnlock()
	t, ok := depInterfaces[key]

	return t, ok
}

func init() {
	RegisterDepInterface("sessionmanager.Trust", reflect.TypeFor[Trust]())
	RegisterDepInterface("sessionmanager.Database", reflect.TypeFor[Database]())
	RegisterDepInterface("sessionmanager.Migrate", reflect.TypeFor[Migrate]())
}

type Module interface {
	Module() ModuleInfo
}

type ModuleInfo struct {
	ID  string
	New func() Module
}

type Provisioner interface {
	Provision(ctx *Context) error
}

type App interface {
	Start() error
	Stop() error
}
