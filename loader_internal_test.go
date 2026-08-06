package sessionmanager

import (
	"reflect"
	"strings"
	"testing"
)

// --- test fixtures -------------------------------------------------------

// depTestIface is an interface used only by loader tests. It is registered
// once (below) under a stable key so struct-tag dep resolution can find it.
type depTestIface interface{ isDepTestTarget() }

type depTestTarget struct{ stubID string }

func (d *depTestTarget) Module() ModuleInfo {
	return ModuleInfo{ID: d.stubID, New: func() Module { return d }}
}
func (d *depTestTarget) isDepTestTarget() {}

// depTestConsumer has one dep-tagged field pointing at a depTestIface target.
type depTestConsumer struct {
	stubID  string
	Dep     string `dep:"test.loader.Iface"`
	private string //nolint:unused // exercises the unexported-field skip path
}

func (d *depTestConsumer) Module() ModuleInfo {
	return ModuleInfo{ID: d.stubID, New: func() Module { return d }}
}

// plainModule implements only Module (does NOT implement depTestIface), used
// for the wrong-type validation case.
type plainModule struct{ stubID string }

func (p *plainModule) Module() ModuleInfo {
	return ModuleInfo{ID: p.stubID, New: func() Module { return p }}
}

func init() {
	// Registered once for the whole test binary.
	RegisterDepInterface("test.loader.Iface", reflect.TypeFor[depTestIface]())
}

// --- readDeps ------------------------------------------------------------

func TestReadDeps_FindsTaggedSkipsRest(t *testing.T) {
	p := &pendingModule{
		id:  "consumer",
		mod: &depTestConsumer{stubID: "consumer", Dep: "target"},
	}
	edges, err := readDeps(p)
	if err != nil {
		t.Fatalf("readDeps returned error: %v", err)
	}
	if len(edges) != 1 {
		t.Fatalf("expected 1 edge, got %d: %+v", len(edges), edges)
	}
	if edges[0].targetID != "target" || edges[0].field != "Dep" {
		t.Fatalf("unexpected edge: %+v", edges[0])
	}
	if edges[0].iface != reflect.TypeFor[depTestIface]() {
		t.Fatalf("edge iface not resolved to depTestIface: %v", edges[0].iface)
	}
}

func TestReadDeps_MissingRequiredDep(t *testing.T) {
	// Empty dep field must be reported as an error.
	p := &pendingModule{
		id:  "consumer",
		mod: &depTestConsumer{stubID: "consumer", Dep: ""},
	}
	_, err := readDeps(p)
	if err == nil {
		t.Fatal("expected error for empty required dep, got nil")
	}
	if !strings.Contains(err.Error(), "Dep") {
		t.Fatalf("error should name the field Dep: %v", err)
	}
}

// --- validateGraph -------------------------------------------------------

func TestValidateGraph_MisWired(t *testing.T) {
	// Node A depends on a missing ID; node B depends on a present-but-wrong-type
	// module. validateGraph must report BOTH (aggregation, not fail-fast).
	consumerMissing := &pendingModule{
		id:  "a",
		mod: &depTestConsumer{stubID: "a", Dep: "does-not-exist"},
	}
	consumerWrong := &pendingModule{
		id:  "b",
		mod: &depTestConsumer{stubID: "b", Dep: "wrongtype"},
	}
	wrong := &pendingModule{id: "wrongtype", mod: &plainModule{stubID: "wrongtype"}}

	pending := []*pendingModule{consumerMissing, consumerWrong, wrong}
	for _, p := range pending {
		deps, err := readDeps(p)
		if err != nil {
			t.Fatalf("readDeps(%s): %v", p.id, err)
		}
		p.deps = deps
	}

	err := validateGraph(pending, map[string]Module{})
	if err == nil {
		t.Fatal("expected validation error, got nil")
	}
	msg := err.Error()
	if !strings.Contains(msg, "does-not-exist") {
		t.Errorf("expected missing-dependency error, got: %v", msg)
	}
	if !strings.Contains(msg, "does not implement") {
		t.Errorf("expected interface-mismatch error, got: %v", msg)
	}
}

func TestValidateGraph_Success(t *testing.T) {
	consumer := &pendingModule{id: "c", mod: &depTestConsumer{stubID: "c", Dep: "t"}}
	target := &pendingModule{id: "t", mod: &depTestTarget{stubID: "t"}}
	pending := []*pendingModule{consumer, target}
	for _, p := range pending {
		deps, _ := readDeps(p)
		p.deps = deps
	}
	if err := validateGraph(pending, map[string]Module{}); err != nil {
		t.Fatalf("expected clean validation, got: %v", err)
	}
}

// --- detectCycle ---------------------------------------------------------

func TestDetectCycle_ReportsPath(t *testing.T) {
	// a -> b -> a via dep edges.
	a := &pendingModule{id: "a", deps: []depEdge{{field: "Dep", targetID: "b"}}}
	b := &pendingModule{id: "b", deps: []depEdge{{field: "Dep", targetID: "a"}}}
	cycle := detectCycle([]*pendingModule{a, b})
	if cycle == "" {
		t.Fatal("expected a cycle to be detected")
	}
	if !strings.Contains(cycle, "a") || !strings.Contains(cycle, "b") {
		t.Fatalf("cycle path should name a and b: %q", cycle)
	}
}

func TestDetectCycle_AcyclicReturnsEmpty(t *testing.T) {
	a := &pendingModule{id: "a", deps: []depEdge{{field: "Dep", targetID: "b"}}}
	b := &pendingModule{id: "b"}
	if cycle := detectCycle([]*pendingModule{a, b}); cycle != "" {
		t.Fatalf("expected no cycle, got %q", cycle)
	}
}

// --- prepare (no provision) ----------------------------------------------

// prepareProvisioner records whether Provision ran.
type prepareProvisioner struct {
	stubID      string
	provisioned bool
}

func (m *prepareProvisioner) Module() ModuleInfo {
	return ModuleInfo{ID: m.stubID, New: func() Module { return &prepareProvisioner{stubID: m.stubID} }}
}
func (m *prepareProvisioner) Provision(_ *Context) error { m.provisioned = true; return nil }

type prepareCfg struct{ id string }

func (c *prepareCfg) Module() string                    { return c.id }
func (c *prepareCfg) UnmarshalExtension(_ Module) error { return nil }

func TestPrepare_NoProvision(t *testing.T) {
	id := "prepare/" + t.Name()
	RegisterModule(&prepareProvisioner{stubID: id})

	c, cancel := NewContext(t.Context())
	defer cancel(nil)

	p, err := c.prepare(&prepareCfg{id: id}, false)
	if err != nil {
		t.Fatalf("prepare: %v", err)
	}
	pp, ok := p.mod.(*prepareProvisioner)
	if !ok {
		t.Fatalf("unexpected module type %T", p.mod)
	}
	if pp.provisioned {
		t.Fatal("prepare must NOT provision the module")
	}
	// And it must not be registered into the context yet.
	if _, err := c.GetModule(id); err == nil {
		t.Fatal("prepare must not register the module into the context")
	}
}
