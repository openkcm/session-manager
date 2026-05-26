package sessionmanager

import (
	"context"
	"errors"
	"fmt"
	"io"
	"reflect"
	"slices"

	slogctx "github.com/veqryn/slog-context"
)

type Context struct {
	//nolint:containedctx
	context.Context

	mods     map[string]Module
	modOrder []string
	apps     map[string]App
}

func (c *Context) cloneWithParent(parent context.Context) *Context {
	return &Context{
		Context:  parent,
		mods:     c.mods,
		modOrder: c.modOrder,
		apps:     c.apps,
	}
}

func (c *Context) WithValue(key, val any) *Context {
	return c.cloneWithParent(context.WithValue(c.Context, key, val))
}

func NewContext(ctx context.Context) (*Context, context.CancelCauseFunc) {
	ctx, cancelCause := context.WithCancelCause(ctx)
	c := &Context{
		Context:  ctx,
		mods:     make(map[string]Module),
		modOrder: nil,
		apps:     make(map[string]App),
	}
	return c, func(cause error) {
		cancelCause(cause)
		for name, app := range c.apps {
			if closer, ok := app.(io.Closer); ok {
				if err := closer.Close(); err != nil {
					slogctx.Error(c, "failed to close an app", "app", name, "error", err)
				}
			}
		}
		for _, v := range slices.Backward(c.modOrder) {
			id := v
			mod, ok := c.mods[id]
			if !ok {
				continue
			}
			if closer, ok := mod.(io.Closer); ok {
				if err := closer.Close(); err != nil {
					slogctx.Error(c, "failed to close a module", "module", id, "error", err)
				}
			}
		}
	}
}

type ExtensionConfig interface {
	Module() string
	UnmarshalExtension(into Module) error
}

func (c *Context) GetModule(id string) (Module, error) {
	if mod, ok := c.mods[id]; ok {
		return mod, nil
	}

	return nil, errors.New("module is not loaded")
}

func (c *Context) GetApp(id string) (App, error) {
	if app, ok := c.apps[id]; ok {
		return app, nil
	}

	return nil, errors.New("app is not loaded")
}

func (c *Context) LoadModule(cfg ExtensionConfig) (Module, error) {
	before := len(c.modOrder)
	mod, modInfo, err := c.instantiate(cfg)
	if err != nil {
		c.unloadModulesAfter(before)
		return nil, err
	}

	if _, ok := c.mods[modInfo.ID]; ok {
		c.unloadModulesAfter(before)
		return nil, errors.New("module has already been loaded")
	}

	c.mods[modInfo.ID] = mod
	c.modOrder = append(c.modOrder, modInfo.ID)

	return mod, nil
}

func (c *Context) LoadApp(cfg ExtensionConfig) (App, error) {
	before := len(c.modOrder)
	mod, modInfo, err := c.instantiate(cfg)
	if err != nil {
		c.unloadModulesAfter(before)
		return nil, err
	}

	app, ok := mod.(App)
	if !ok {
		c.unloadModulesAfter(before)
		return nil, fmt.Errorf("module %q does not implement the App interface", modInfo.ID)
	}

	if _, ok := c.apps[modInfo.ID]; ok {
		c.unloadModulesAfter(before)
		return nil, errors.New("app has already been loaded")
	}

	c.apps[modInfo.ID] = app

	return app, nil
}

// unloadModulesAfter rolls back any modules appended to modOrder at or after
// the snapshot index. Modules are closed in reverse load order. It is the
// recovery path for a failed LoadModule or LoadApp call: every successfully
// loaded child module is closed and removed from the registry before the
// error surfaces to the caller.
func (c *Context) unloadModulesAfter(snapshot int) {
	if snapshot >= len(c.modOrder) {
		return
	}
	for i := len(c.modOrder) - 1; i >= snapshot; i-- {
		id := c.modOrder[i]
		mod, ok := c.mods[id]
		if !ok {
			continue
		}
		if closer, ok := mod.(io.Closer); ok {
			if err := closer.Close(); err != nil {
				slogctx.Error(c, "failed to close a module during rollback", "module", id, "error", err)
			}
		}
		delete(c.mods, id)
	}
	c.modOrder = c.modOrder[:snapshot]
}

// instantiate resolves cfg.Module(), calls New(), unmarshals the extension, and
// runs Provision if the resulting instance is a Provisioner. It is shared by
// LoadModule and LoadApp.
func (c *Context) instantiate(cfg ExtensionConfig) (Module, ModuleInfo, error) {
	modInfo, err := GetModule(cfg.Module())
	if err != nil {
		return nil, ModuleInfo{}, fmt.Errorf("getting module %q: %w", reflect.TypeOf(cfg), err)
	}

	slogctx.Debug(c, "loading module", "module", modInfo.ID)

	mod := modInfo.New()
	rv := reflect.ValueOf(mod)
	if rv.Kind() == reflect.Pointer && rv.Elem().Kind() == reflect.Struct {
		if err := cfg.UnmarshalExtension(mod); err != nil {
			return nil, ModuleInfo{}, fmt.Errorf("unmarshaling extension %s: %w", modInfo.ID, err)
		}
	}

	slogctx.Debug(c, "instantiated a module", "module", modInfo.ID)

	if provisioner, ok := mod.(Provisioner); ok {
		if err := provisioner.Provision(c); err != nil {
			return nil, ModuleInfo{}, fmt.Errorf("provisioning module: %w", err)
		}

		slogctx.Debug(c, "provisioned module", "module", modInfo.ID)
	}

	return mod, modInfo, nil
}
