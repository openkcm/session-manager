package trustmapping_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	trustv1 "github.com/openkcm/api-sdk/proto/kms/api/cmk/trust/v1"

	sessionmanager "github.com/openkcm/session-manager"
	tmmod "github.com/openkcm/session-manager/modules/grpc/trustmapping"
	_ "github.com/openkcm/session-manager/modules/standard"
)

// stubTrust satisfies sessionmanager.Trust enough for Provision to type-assert
// successfully. No method bodies need to be exercised in these tests.
type stubTrust struct{}

func (stubTrust) Apply(context.Context, *trustv1.Trust) error { return nil }
func (stubTrust) Block(context.Context, string) error         { return nil }
func (stubTrust) Remove(context.Context, string) error        { return nil }
func (stubTrust) Unblock(context.Context, string) error       { return nil }

var errStubTrustGet = errors.New("stub get not implemented")

func (stubTrust) Get(context.Context, string) (*trustv1.Trust, error) {
	return nil, errStubTrustGet
}

// stubTrustModule lets us register a fake trust module with the registry under
// a custom ID for tests.
type stubTrustModule struct {
	stubTrust

	id string
}

func (s *stubTrustModule) Module() sessionmanager.ModuleInfo {
	return sessionmanager.ModuleInfo{
		ID:  s.id,
		New: func() sessionmanager.Module { return s },
	}
}

func TestModule_Registration(t *testing.T) {
	info, err := sessionmanager.GetModule("service.module.grpc.trustmapping")
	require.NoError(t, err)
	assert.Equal(t, "service.module.grpc.trustmapping", info.ID)
}

func TestModule_ProvisionResolvesCustomTrust(t *testing.T) {
	id := "trust.module.test." + t.Name()
	sessionmanager.RegisterModule(&stubTrustModule{id: id})

	ctx, cancel := sessionmanager.NewContext(t.Context())
	defer cancel(nil)

	_, err := ctx.LoadModule(&extConfig{moduleID: id})
	require.NoError(t, err)

	m := &tmmod.Module{Trust: id}
	require.NoError(t, m.Provision(ctx))
}

func TestModule_ProvisionMissingTrustFails(t *testing.T) {
	ctx, cancel := sessionmanager.NewContext(t.Context())
	defer cancel(nil)

	m := &tmmod.Module{Trust: "no.such.trust.module"}
	err := m.Provision(ctx)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no.such.trust.module")
}

// extConfig is a minimal sessionmanager.ExtensionConfig used by the test to
// load fake modules through the registry.
type extConfig struct{ moduleID string }

func (c *extConfig) Module() string                                   { return c.moduleID }
func (c *extConfig) UnmarshalExtension(_ sessionmanager.Module) error { return nil }
