package oidc_test

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/docker/go-connections/nat"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/pressly/goose/v3"
	"github.com/testcontainers/testcontainers-go/modules/postgres"

	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/openkcm/session-manager/internal/oidc"
	oidcsql "github.com/openkcm/session-manager/internal/oidc/sql"
	migrations "github.com/openkcm/session-manager/sql"
)

const (
	DBHost     = "localhost"
	DBUser     = "postgres"
	DBPassword = "secret"
	DBName     = "session_manager"
	DBSSLMode  = "disable"
)

type RepoWrapper struct {
	Repo             oidc.ProviderRepository
	MockGetForTenant func(ctx context.Context, tenantID string) (oidc.Provider, error)
	MockUpdate       func(ctx context.Context, tenantID string, provider oidc.Provider) error
	MockDelete       func(ctx context.Context, tenantID string, provider oidc.Provider) error
}

var _ oidc.ProviderRepository = &RepoWrapper{}

// Create implements oidc.ProviderRepository.
func (m *RepoWrapper) Create(ctx context.Context, tenantID string, provider oidc.Provider) error {
	return m.Repo.Create(ctx, tenantID, provider)
}

// Delete implements oidc.ProviderRepository.
func (m *RepoWrapper) Delete(ctx context.Context, tenantID string, provider oidc.Provider) error {
	if m.MockDelete != nil {
		err := m.MockDelete(ctx, tenantID, provider)
		if err != nil {
			return err
		}
	}

	return m.Repo.Delete(ctx, tenantID, provider)
}

// Get implements oidc.ProviderRepository.
func (m *RepoWrapper) Get(ctx context.Context, issuerURL string) (oidc.Provider, error) {
	return m.Repo.Get(ctx, issuerURL)
}

// GetForTenant implements oidc.ProviderRepository.
func (m *RepoWrapper) GetForTenant(ctx context.Context, tenantID string) (oidc.Provider, error) {
	if m.MockGetForTenant != nil {
		_, err := m.MockGetForTenant(ctx, tenantID)
		if err != nil {
			return oidc.Provider{}, err
		}
	}
	return m.Repo.GetForTenant(ctx, tenantID)
}

// Update implements oidc.ProviderRepository.
func (m *RepoWrapper) Update(ctx context.Context, tenantID string, provider oidc.Provider) error {
	if m.MockUpdate != nil {
		err := m.MockUpdate(ctx, tenantID, provider)
		if err != nil {
			return err
		}
	}
	return m.Repo.Update(ctx, tenantID, provider)
}

func createRepo(ctx context.Context) (oidc.ProviderRepository, error) {
	pgContainer, err := postgres.Run(
		ctx,
		"postgres:17-alpine",
		postgres.WithDatabase(DBName),
		postgres.WithUsername(DBUser),
		postgres.WithPassword(DBPassword),
		postgres.BasicWaitStrategies(),
	)
	if err != nil {
		return nil, err
	}

	port, err := pgContainer.MappedPort(ctx, nat.Port("5432"))
	if err != nil {
		return nil, err
	}

	connStr := fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%s sslmode=%s", DBHost, DBUser, DBPassword, DBName, port.Port(), DBSSLMode)

	dbPool, err := pgxpool.New(ctx, connStr)
	if err != nil {
		panic(err)
	}

	err = migrateDB(ctx, connStr)
	if err != nil {
		return nil, err
	}

	return oidcsql.NewRepository(dbPool), nil
}

func migrateDB(ctx context.Context, connStr string) error {
	db, err := sql.Open("pgx", connStr)
	if err != nil {
		return err
	}

	defer db.Close()

	goose.SetBaseFS(migrations.FS)
	if err := goose.SetDialect("pgx"); err != nil {
		return err
	}

	if err := goose.UpContext(ctx, db, "."); err != nil {
		return err
	}
	return nil
}
