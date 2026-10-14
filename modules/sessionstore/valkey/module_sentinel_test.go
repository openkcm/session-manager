package valkey_test

import (
	"context"
	"net/netip"
	"testing"
	"time"

	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/api/types/network"
	"github.com/openkcm/common-sdk/pkg/commoncfg"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/openkcm/session-manager/internal/session"
	sessionstorevalkey "github.com/openkcm/session-manager/modules/sessionstore/valkey"
)

// Sentinel master set name and the fixed ports used by startValkeySentinel.
const (
	sentinelMasterSet  = "mymaster"
	sentinelMasterPort = "6380"
	sentinelPort       = "26379"
)

func embedded(value string) commoncfg.SourceRef {
	return commoncfg.SourceRef{Source: commoncfg.EmbeddedSourceValue, Value: value}
}

// startValkeySentinel spins up a single container running a master and a
// sentinel that monitors it, and returns the host:port of the sentinel.
//
// A sentinel-mode client connects to the sentinel, asks it for the master
// address, and then connects to that address — so the master address the
// sentinel advertises must be reachable from the test process. We mirror
// valkey-go's own test setup: the sentinel monitors the master at a loopback
// address with a fixed port, and both container ports are bound to identical
// host ports so 127.0.0.1:<masterPort> resolves to the same container inside
// and outside Docker.
func startValkeySentinel(t *testing.T) string {
	t.Helper()
	ctx := context.Background()

	// Master in the background, then sentinel in the foreground monitoring it on
	// the loopback address with the fixed master port that matches the host
	// binding below.
	script := "valkey-server --save '' --appendonly no --port " + sentinelMasterPort + " &\n" +
		"printf 'sentinel monitor " + sentinelMasterSet + " 127.0.0.1 " + sentinelMasterPort + " 1\\n' > /tmp/sentinel.conf\n" +
		"exec valkey-server /tmp/sentinel.conf --sentinel"

	masterTCP := sentinelMasterPort + "/tcp"
	sentinelTCP := sentinelPort + "/tcp"
	loopback := netip.MustParseAddr("127.0.0.1")

	ctr, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		Image:        "valkey/valkey:8-alpine",
		Entrypoint:   []string{"/bin/sh", "-c", script},
		ExposedPorts: []string{masterTCP, sentinelTCP},
		HostConfigModifier: func(hc *container.HostConfig) {
			hc.PortBindings = network.PortMap{
				network.MustParsePort(masterTCP):   []network.PortBinding{{HostIP: loopback, HostPort: sentinelMasterPort}},
				network.MustParsePort(sentinelTCP): []network.PortBinding{{HostIP: loopback, HostPort: sentinelPort}},
			}
		},
		WaitingFor: wait.ForAll(
			wait.ForListeningPort(masterTCP),
			wait.ForListeningPort(sentinelTCP),
		).WithDeadline(60 * time.Second),
		Started: true,
	})
	require.NoError(t, err)
	t.Cleanup(func() { _ = ctr.Terminate(ctx) })

	host, err := ctr.Host(ctx)
	require.NoError(t, err)
	return host + ":" + sentinelPort
}

// TestModule_ProvisionSentinelInvalidTopologyRefreshInterval asserts the
// sentinel fields (MasterSet and TopologyRefreshInterval) are plumbed from the
// module config into the valkey client's SentinelOption. A negative
// TopologyRefreshInterval is rejected eagerly by valkey.NewClient, but only on
// the sentinel path, which is taken only when MasterSet is set — so this error
// confirms both fields are actually wired through. No container needed.
func TestModule_ProvisionSentinelInvalidTopologyRefreshInterval(t *testing.T) {
	m := new(sessionstorevalkey.Module)
	m.Host = embedded("localhost:26379")
	m.User = embedded("")
	m.Password = embedded("")
	m.MasterSet = sentinelMasterSet
	m.TopologyRefreshInterval = -time.Second

	err := m.Provision(nil)
	require.ErrorContains(t, err, "creating valkey client")
	require.ErrorContains(t, err, "TopologyRefreshInterval")
}

// TestModule_ProvisionSentinelRoundTrip provisions the module against a real
// sentinel and performs a state store/load round-trip, proving the client
// resolved the master through the sentinel and reached it.
func TestModule_ProvisionSentinelRoundTrip(t *testing.T) {
	addr := startValkeySentinel(t)

	m := new(sessionstorevalkey.Module)
	m.Host = embedded(addr)
	m.User = embedded("")
	m.Password = embedded("")
	m.Prefix = "sentinel-test"
	m.MasterSet = sentinelMasterSet

	require.NoError(t, m.Provision(nil))
	t.Cleanup(func() { _ = m.Close() })

	ctx := context.Background()
	want := session.State{ID: "state-1", TenantID: "tenant-1", Expiry: time.Now().Add(time.Hour)}
	require.NoError(t, m.StoreState(ctx, want))

	got, err := m.LoadState(ctx, want.ID)
	require.NoError(t, err)
	assert.Equal(t, want.ID, got.ID)
	assert.Equal(t, want.TenantID, got.TenantID)
}

// TestModule_ProvisionSentinelTopologyRefresh provisions the module against a
// real sentinel with periodic topology refresh enabled and verifies a state
// round-trip still succeeds (the background reconciler must not interfere).
func TestModule_ProvisionSentinelTopologyRefresh(t *testing.T) {
	addr := startValkeySentinel(t)

	m := new(sessionstorevalkey.Module)
	m.Host = embedded(addr)
	m.User = embedded("")
	m.Password = embedded("")
	m.Prefix = "sentinel-refresh"
	m.MasterSet = sentinelMasterSet
	m.TopologyRefreshInterval = time.Second

	require.NoError(t, m.Provision(nil))
	t.Cleanup(func() { _ = m.Close() })

	ctx := context.Background()
	want := session.State{ID: "state-2", TenantID: "tenant-2", Expiry: time.Now().Add(time.Hour)}
	require.NoError(t, m.StoreState(ctx, want))

	// Let the reconciler tick at least once, then confirm the client is still
	// bound to a reachable master.
	time.Sleep(1500 * time.Millisecond)
	got, err := m.LoadState(ctx, want.ID)
	require.NoError(t, err)
	assert.Equal(t, want.ID, got.ID)
}
