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

// GetModuleAs looks up a loaded module by ID and asserts it to the interface T.
// It is the type-safe replacement for GetModule.
func GetModuleAs[T any](c *Context, id string) (T, error) {
	var zero T

	mod, err := c.GetModule(id)
	if err != nil {
		return zero, err
	}

	typed, ok := mod.(T)
	if !ok {
		return zero, fmt.Errorf("module %q does not implement %s", id, reflect.TypeFor[T]())
	}

	return typed, nil
}

// unloadModulesAfter rolls back any modules appended to modOrder at or after
// the snapshot index. Modules are closed in reverse load order. It is the
// recovery path for a failed LoadAll call: every successfully provisioned
// module is closed and removed from the registry before the error surfaces to
// the caller.
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
