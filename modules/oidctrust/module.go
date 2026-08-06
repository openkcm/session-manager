package oidctrust

import (
	"fmt"

	sessionmanager "github.com/openkcm/session-manager"
	sqltrust "github.com/openkcm/session-manager/modules/oidctrust/internal/sql"
)

const moduleID = "trust.module.oidc"

func newModule() sessionmanager.Module {
	return new(TrustModule)
}

func init() {
	sessionmanager.RegisterModule(new(TrustModule))
}

// TrustModule is a module that implements sessionmanager.Trust interface. It's using a database provided by the
// [dbModule] module which implements sessionmanager.DBModule.
type TrustModule struct {
	DBModule string `yaml:"dbModule" default:"database.module.pgxpool"`

	repository TrustRepository
}

func (m *TrustModule) Module() sessionmanager.ModuleInfo {
	return sessionmanager.ModuleInfo{
		ID:  moduleID,
		New: newModule,
	}
}

func (m *TrustModule) Provision(ctx *sessionmanager.Context) error {
	db, err := sessionmanager.GetModuleAs[sessionmanager.Database](ctx, m.DBModule)
	if err != nil {
		return fmt.Errorf("getting db module: %w", err)
	}

	m.repository = sqltrust.NewRepository(db)

	return nil
}

var _ sessionmanager.Trust = (*TrustModule)(nil)
