// Package valkey provides the sessionstore.module.valkey module: a
// session.Repository backed by Valkey, configured by the top-level valkey:
// config block. It registers itself with the sessionmanager module registry
// at init time and is loaded by business.Main as a top-level dependency.
package valkey

import (
	"fmt"
	"strings"
	"time"

	"github.com/openkcm/common-sdk/pkg/commoncfg"
	"github.com/valkey-io/valkey-go"

	sessionmanager "github.com/openkcm/session-manager"
	"github.com/openkcm/session-manager/internal/session"
	sessionvalkey "github.com/openkcm/session-manager/internal/session/valkey"
)

const moduleID = "sessionstore.module.valkey"

func init() {
	sessionmanager.RegisterModule(new(Module))
}

func newModule() sessionmanager.Module {
	return new(Module)
}

// Module is the sessionstore.module.valkey module. It owns a Valkey client
// and exposes a session.Repository backed by it.
type Module struct {
	*sessionvalkey.Repository

	Mod                     string              `yaml:"module"`
	Host                    commoncfg.SourceRef `yaml:"host"`
	User                    commoncfg.SourceRef `yaml:"user"`
	Password                commoncfg.SourceRef `yaml:"password"`
	Prefix                  string              `yaml:"prefix"`
	SecretRef               commoncfg.SecretRef `yaml:"secretRef"`
	MasterSet               string              `yaml:"masterSet"`
	TopologyRefreshInterval time.Duration       `yaml:"topologyRefreshInterval"`

	client valkey.Client
}

func (m *Module) Module() sessionmanager.ModuleInfo {
	return sessionmanager.ModuleInfo{
		ID:  moduleID,
		New: newModule,
	}
}

func (m *Module) Provision(_ *sessionmanager.Context) error {
	host, err := commoncfg.LoadValueFromSourceRef(m.Host)
	if err != nil {
		return fmt.Errorf("loading valkey host: %w", err)
	}
	hosts := strings.Split(string(host), ",")
	user, err := commoncfg.LoadValueFromSourceRef(m.User)
	if err != nil {
		return fmt.Errorf("loading valkey user: %w", err)
	}
	password, err := commoncfg.LoadValueFromSourceRef(m.Password)
	if err != nil {
		return fmt.Errorf("loading valkey password: %w", err)
	}

	opts := valkey.ClientOption{
		InitAddress: hosts,
		Username:    string(user),
		Password:    string(password),
	}

	if m.SecretRef.Type == commoncfg.MTLSSecretType {
		tlsConfig, err := commoncfg.LoadMTLSConfig(&m.SecretRef.MTLS)
		if err != nil {
			return fmt.Errorf("loading valkey mTLS config: %w", err)
		}
		opts.TLSConfig = tlsConfig
	}

	opts.Sentinel = valkey.SentinelOption{
		MasterSet:               m.MasterSet,
		TopologyRefreshInterval: m.TopologyRefreshInterval,
		Username:                opts.Username,
		Password:                opts.Password,
		TLSConfig:               opts.TLSConfig,
	}

	client, err := valkey.NewClient(opts)
	if err != nil {
		return fmt.Errorf("creating valkey client: %w", err)
	}
	m.client = client
	m.Repository = sessionvalkey.NewRepository(client, m.Prefix)

	return nil
}

func (m *Module) Close() error {
	if m.client == nil {
		return nil
	}
	m.client.Close()
	return nil
}

// Compile-time guarantee that the module satisfies session.Repository via the
// embedded *sessionvalkey.Repository.
var _ session.Repository = (*Module)(nil)
