package sessionmanager

import (
	"fmt"
	"iter"
	"maps"
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
