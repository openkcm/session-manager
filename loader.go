package sessionmanager

import (
	"errors"
	"fmt"
	"reflect"
	"strings"

	slogctx "github.com/veqryn/slog-context"
)

// LoadSpec describes one module (or app) to load, together with any child
// modules an app owns (e.g. a grpc server app and its service modules).
type LoadSpec struct {
	Cfg      ExtensionConfig
	IsApp    bool
	Children []LoadSpec
}

// depEdge is a resolved dependency of a pending module: the target module ID
// named by a dep-tagged field and the interface that target must implement.
type depEdge struct {
	field    string // struct field name, for error messages
	targetID string // module ID the dep field points at
	iface    reflect.Type
}

// pendingModule is a module that has been instantiated and had its config
// unmarshaled, but not yet provisioned.
type pendingModule struct {
	id         string
	info       ModuleInfo
	cfg        ExtensionConfig
	mod        Module
	isApp      bool
	deps       []depEdge
	structDeps []string // module IDs this node structurally contains (app -> services)
}

// LoadAll loads a set of module specs through four phases so the dependency
// graph can be validated before any module is provisioned (before side
// effects such as opening database pools):
//
// Phase 1:
// flatten specs into pending nodes, recording app->service edges.
//
// Phase 2:
// instantiate + UnmarshalExtension each node (NO Provision yet).
//
// Phase 3:
// read dep-tagged fields via reflection and validate the whole graph
// (every referenced ID present, satisfies its interface, no cycles),
// aggregating all errors.
//
// Phase 4:
// provision each node in dependency order (derived by topological sort over
// the dep-tagged and app->service edges) and register it into the Context,
// rolling back on failure.
func (c *Context) LoadAll(specs []LoadSpec) error {
	// Phases 1-3
	pending, err := c.validateAll(specs)
	if err != nil {
		return err
	}

	ordered := topoSort(pending)

	// Phase 4
	before := len(c.modOrder)
	for _, p := range ordered {
		if err := c.provisionAndRegister(p); err != nil {
			c.unloadModulesAfter(before)
			return fmt.Errorf("loading module %q: %w", p.id, err)
		}
	}

	return nil
}

// ValidateAll runs all phases without provisioning anything.
// It lets an operator or a test check that every referenced module registered,
// every dependency satisfying its declared interface, no cycles.
func (c *Context) ValidateAll(specs []LoadSpec) error {
	_, err := c.validateAll(specs)
	return err
}

// validateAll performs Phases 1–4 and returns the prepared (but not
// provisioned) nodes for LoadAll to finish, or an aggregated validation error.
func (c *Context) validateAll(specs []LoadSpec) ([]*pendingModule, error) {
	pending, err := c.prepareAll(specs)
	if err != nil {
		return nil, err
	}

	for _, p := range pending {
		deps, derr := readDeps(p)
		if derr != nil {
			return nil, fmt.Errorf("reading dependencies of %q: %w", p.id, derr)
		}
		p.deps = deps
	}

	if verr := validateGraph(pending, c.mods); verr != nil {
		return nil, fmt.Errorf("validating module graph: %w", verr)
	}

	return pending, nil
}

// prepareAll flattens specs depth-first into pending nodes and
// prepares each. Order is preserved.
func (c *Context) prepareAll(specs []LoadSpec) ([]*pendingModule, error) {
	var pending []*pendingModule
	seen := make(map[string]bool)

	var walk func(specs []LoadSpec) error
	walk = func(specs []LoadSpec) error {
		for _, spec := range specs {
			p, err := c.prepare(spec.Cfg, spec.IsApp)
			if err != nil {
				return err
			}

			if seen[p.id] {
				return fmt.Errorf("module %q appears more than once in the load set", p.id)
			}
			seen[p.id] = true

			for _, child := range spec.Children {
				p.structDeps = append(p.structDeps, child.Cfg.Module())
			}

			pending = append(pending, p)

			if len(spec.Children) > 0 {
				if err := walk(spec.Children); err != nil {
					return err
				}
			}
		}
		return nil
	}

	if err := walk(specs); err != nil {
		return nil, err
	}

	return pending, nil
}

// prepare resolves cfg.Module(), instantiates it, and unmarshals its config
// but doesn't provision it.
func (c *Context) prepare(cfg ExtensionConfig, isApp bool) (*pendingModule, error) {
	modInfo, err := GetModule(cfg.Module())
	if err != nil {
		return nil, fmt.Errorf("getting module %q: %w", reflect.TypeOf(cfg), err)
	}

	mod := modInfo.New()
	rv := reflect.ValueOf(mod)
	if rv.Kind() == reflect.Pointer && rv.Elem().Kind() == reflect.Struct {
		if err := cfg.UnmarshalExtension(mod); err != nil {
			return nil, fmt.Errorf("unmarshaling extension %s: %w", modInfo.ID, err)
		}
	}

	return &pendingModule{
		id:    modInfo.ID,
		info:  modInfo,
		cfg:   cfg,
		mod:   mod,
		isApp: isApp,
	}, nil
}

// provisionAndRegister provisions a prepared module (if it is a Provisioner)
// and registers it into the Context.
func (c *Context) provisionAndRegister(p *pendingModule) error {
	if provisioner, ok := p.mod.(Provisioner); ok {
		if err := provisioner.Provision(c); err != nil {
			return fmt.Errorf("provisioning module: %w", err)
		}
		slogctx.Debug(c, "provisioned module", "module", p.id)
	}

	if _, ok := c.mods[p.id]; ok {
		return errors.New("module has already been loaded")
	}
	c.mods[p.id] = p.mod
	c.modOrder = append(c.modOrder, p.id)

	if p.isApp {
		app, ok := p.mod.(App)
		if !ok {
			return fmt.Errorf("module %q does not implement the App interface", p.id)
		}
		c.apps[p.id] = app
	}

	return nil
}

// readDeps reflects over a module's exported, dep-tagged string fields and
// resolves each to a depEdge. Unexported and untagged fields are skipped.
func readDeps(p *pendingModule) ([]depEdge, error) {
	rv := reflect.Indirect(reflect.ValueOf(p.mod))
	if rv.Kind() != reflect.Struct {
		return nil, nil
	}

	rt := rv.Type()
	var edges []depEdge
	var errs []error

	for i := range rt.NumField() {
		field := rt.Field(i)
		if field.PkgPath != "" { // unexported
			continue
		}
		key := field.Tag.Get("dep")
		if key == "" {
			continue
		}

		if field.Type.Kind() != reflect.String {
			errs = append(errs, fmt.Errorf("field %s has a dep tag but is not a string", field.Name))
			continue
		}

		iface, ok := lookupDepInterface(key)
		if !ok {
			errs = append(errs, fmt.Errorf("field %s references unknown dep interface %q", field.Name, key))
			continue
		}

		targetID := rv.Field(i).String()
		if targetID == "" {
			errs = append(errs, fmt.Errorf("field %s (dep %q) is empty", field.Name, key))
			continue
		}

		edges = append(edges, depEdge{field: field.Name, targetID: targetID, iface: iface})
	}

	return edges, errors.Join(errs...)
}

// validateGraph checks the whole set of pending modules at once and aggregates
// every problem into a single joined error. It verifies, for each dep edge, that
// the target is present and implements the required interface, and that there are
// no dependency cycles.
func validateGraph(pending []*pendingModule, alreadyLoaded map[string]Module) error {
	byID := make(map[string]*pendingModule, len(pending))
	for _, p := range pending {
		byID[p.id] = p
	}

	// resolveInstance returns the module instance for an ID, whether it is in
	// this pending set or already loaded in the context.
	resolveInstance := func(id string) (Module, bool) {
		if p, ok := byID[id]; ok {
			return p.mod, true
		}
		if m, ok := alreadyLoaded[id]; ok {
			return m, true
		}
		return nil, false
	}

	var errs []error

	for _, p := range pending {
		for _, e := range p.deps {
			target, ok := resolveInstance(e.targetID)
			if !ok {
				errs = append(errs, fmt.Errorf(
					"module %q field %s depends on %q which is not loaded",
					p.id, e.field, e.targetID))
				continue
			}
			if !reflect.TypeOf(target).Implements(e.iface) {
				errs = append(errs, fmt.Errorf(
					"module %q field %s requires %s but %q (%T) does not implement it",
					p.id, e.field, e.iface, e.targetID, target))
			}
		}

		// Structural (app -> service) edges must also be present.
		for _, childID := range p.structDeps {
			if _, ok := resolveInstance(childID); !ok {
				errs = append(errs, fmt.Errorf(
					"app %q contains service %q which is not loaded", p.id, childID))
			}
		}
	}

	if cycle := detectCycle(pending); cycle != "" {
		errs = append(errs, fmt.Errorf("dependency cycle: %s", cycle))
	}

	return errors.Join(errs...)
}

// detectCycle runs a three-color DFS over the pending-node subgraph
// and returns a readable path string for the first cycle found,
// or "" if the graph is acyclic. Edges to targets outside the pending
// set (already loaded, hence acyclic) are ignored.
func detectCycle(pending []*pendingModule) string {
	const (
		white = 0 // unvisited
		gray  = 1 // on the current DFS stack
		black = 2 // fully explored
	)

	byID := make(map[string]*pendingModule, len(pending))
	for _, p := range pending {
		byID[p.id] = p
	}

	color := make(map[string]int, len(pending))
	var stack []string

	edgesOf := func(p *pendingModule) []string {
		ids := make([]string, 0, len(p.deps)+len(p.structDeps))
		for _, e := range p.deps {
			ids = append(ids, e.targetID)
		}
		ids = append(ids, p.structDeps...)
		return ids
	}

	var visit func(id string) string
	visit = func(id string) string {
		p, ok := byID[id]
		if !ok {
			return "" // outside the pending set
		}
		color[id] = gray
		stack = append(stack, id)

		for _, next := range edgesOf(p) {
			switch color[next] {
			case gray:
				// Found a back edge: build the cycle path from the stack.
				start := 0
				for i, s := range stack {
					if s == next {
						start = i
						break
					}
				}
				return strings.Join(append(append([]string{}, stack[start:]...), next), " -> ")
			case white:
				if cyc := visit(next); cyc != "" {
					return cyc
				}
			}
		}

		color[id] = black
		stack = stack[:len(stack)-1]
		return ""
	}

	for _, p := range pending {
		if color[p.id] == white {
			if cyc := visit(p.id); cyc != "" {
				return cyc
			}
		}
	}

	return ""
}

// topoSort orders pending modules so that every module comes after all of its
// dependencies. The graph is assumed acyclic (validateGraph has already rejected cycles).
func topoSort(pending []*pendingModule) []*pendingModule {
	index := make(map[string]int, len(pending))
	for i, p := range pending {
		index[p.id] = i
	}

	// successors[x] = nodes that depend on x (so they follow x); indeg[y] = how
	// many in-batch dependencies y still waits on.
	successors := make(map[string][]string, len(pending))
	indeg := make(map[string]int, len(pending))
	for _, p := range pending {
		indeg[p.id] = 0
	}

	edgesOf := func(p *pendingModule) []string {
		ids := make([]string, 0, len(p.deps)+len(p.structDeps))
		for _, e := range p.deps {
			ids = append(ids, e.targetID)
		}
		ids = append(ids, p.structDeps...)
		return ids
	}

	for _, p := range pending {
		for _, dep := range edgesOf(p) {
			if _, inBatch := index[dep]; !inBatch {
				continue // dependency already loaded; no intra-batch constraint
			}
			successors[dep] = append(successors[dep], p.id)
			indeg[p.id]++
		}
	}

	// ready holds in-degree-zero node ids, kept sorted by input index so the
	// earliest-declared ready node is always emitted next (stable, deterministic).
	var ready []string
	for _, p := range pending {
		if indeg[p.id] == 0 {
			ready = append(ready, p.id)
		}
	}

	byID := make(map[string]*pendingModule, len(pending))
	for _, p := range pending {
		byID[p.id] = p
	}

	ordered := make([]*pendingModule, 0, len(pending))
	for len(ready) > 0 {
		// Pop the ready node with the smallest input index.
		pick := 0
		for i := 1; i < len(ready); i++ {
			if index[ready[i]] < index[ready[pick]] {
				pick = i
			}
		}
		id := ready[pick]
		ready = append(ready[:pick], ready[pick+1:]...)

		ordered = append(ordered, byID[id])

		for _, succ := range successors[id] {
			indeg[succ]--
			if indeg[succ] == 0 {
				ready = append(ready, succ)
			}
		}
	}

	// If a cycle slipped through, append any unemitted nodes in input order so we never silently drop them.
	if len(ordered) != len(pending) {
		emitted := make(map[string]bool, len(ordered))
		for _, p := range ordered {
			emitted[p.id] = true
		}
		for _, p := range pending {
			if !emitted[p.id] {
				ordered = append(ordered, p)
			}
		}
	}

	return ordered
}
