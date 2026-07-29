package sessionmanager

import (
	"context"
	"errors"
	"fmt"
	"io"
	"reflect"

	slogctx "github.com/veqryn/slog-context"
)

type Context struct {
	//nolint:containedctx
	context.Context

	mods map[string]Module
}

func (c *Context) cloneWithParent(parent context.Context) *Context {
	return &Context{
		Context: parent,
		mods:    c.mods,
	}
}

func (c *Context) WithValue(key, val any) *Context {
	return c.cloneWithParent(context.WithValue(c.Context, key, val))
}

func NewContext(ctx context.Context) (*Context, context.CancelCauseFunc) {
	ctx, cancelCause := context.WithCancelCause(ctx)
	c := &Context{Context: ctx, mods: make(map[string]Module)}
	return c, func(cause error) {
		cancelCause(cause)
		for name, mod := range c.mods {
			if closer, ok := mod.(io.Closer); ok {
				if err := closer.Close(); err != nil {
					slogctx.Error(c, "failed to close a module", "module", name, "error", err)
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

func (c *Context) LoadModule(cfg ExtensionConfig) (Module, error) {
	modInfo, err := GetModule(cfg.Module())
	if err != nil {
		return nil, fmt.Errorf("getting module %q: %w", reflect.TypeOf(cfg), err)
	}

	if _, ok := c.mods[modInfo.ID]; ok {
		return nil, errors.New("module has already been loaded")
	}

	slogctx.Debug(c, "loading module", "module", modInfo.ID)

	mod := modInfo.New()
	rv := reflect.ValueOf(mod)
	if rv.Kind() == reflect.Pointer && rv.Elem().Kind() == reflect.Struct {
		if err := cfg.UnmarshalExtension(mod); err != nil {
			return nil, fmt.Errorf("unmarshaling extension %s: %w", modInfo.ID, err)
		}
	}

	slogctx.Debug(c, "instantinated module", "module", modInfo.ID)

	if provisioner, ok := mod.(Provisioner); ok {
		if err := provisioner.Provision(c); err != nil {
			return nil, fmt.Errorf("provisioning module: %w", err)
		}

		slogctx.Debug(c, "provisioned module", "module", modInfo.ID)
	}

	c.mods[modInfo.ID] = mod

	return mod, nil
}
