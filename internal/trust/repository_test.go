package trust_test

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/docker/go-connections/nat"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/pressly/goose/v3"
	"github.com/testcontainers/testcontainers-go/modules/postgres"

	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/openkcm/session-manager/internal/trust"
	"github.com/openkcm/session-manager/internal/trust/trustsql"
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
	Repo       trust.OIDCMappingRepository
	MockGet    func(ctx context.Context, tenantID string) (trust.OIDCMapping, error)
	MockCreate func(ctx context.Context, tenantID string, mapping trust.OIDCMapping) error
	MockUpdate func(ctx context.Context, tenantID string, mapping trust.OIDCMapping) error
	MockDelete func(ctx context.Context, tenantID string) error
}

var _ trust.OIDCMappingRepository = &RepoWrapper{}

// Create implements oidc.OIDCMappingRepository.
func (m *RepoWrapper) Create(ctx context.Context, tenantID string, mapping trust.OIDCMapping) error {
	if m.MockCreate != nil {
		err := m.MockCreate(ctx, tenantID, mapping)
		if err != nil {
			return err
		}
	}

	return m.Repo.Create(ctx, tenantID, mapping)
}

// Delete implements oidc.OIDCMappingRepository.
func (m *RepoWrapper) Delete(ctx context.Context, tenantID string) error {
	if m.MockDelete != nil {
		err := m.MockDelete(ctx, tenantID)
		if err != nil {
			return err
		}
	}

	return m.Repo.Delete(ctx, tenantID)
}

// Get implements oidc.OIDCMappingRepository.
func (m *RepoWrapper) Get(ctx context.Context, tenantID string) (trust.OIDCMapping, error) {
	if m.MockGet != nil {
		_, err := m.MockGet(ctx, tenantID)
		if err != nil {
			return trust.OIDCMapping{}, err
		}
	}
	return m.Repo.Get(ctx, tenantID)
}

// Update implements oidc.OIDCMappingRepository.
func (m *RepoWrapper) Update(ctx context.Context, tenantID string, mapping trust.OIDCMapping) error {
	if m.MockUpdate != nil {
		err := m.MockUpdate(ctx, tenantID, mapping)
		if err != nil {
			return err
		}
	}
	return m.Repo.Update(ctx, tenantID, mapping)
}

func createRepo(ctx context.Context) (trust.OIDCMappingRepository, error) {
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

	return trustsql.NewRepository(dbPool), nil
}

func migrateDB(ctx context.Context, connStr string) error {
	db, err := sql.Open("pgx", connStr)
	if err != nil {
		return err
	}

	defer db.Close()

	goose.SetBaseFS(migrations.FS)
	err = goose.SetDialect("pgx")
	if err != nil {
		return err
	}

	err = goose.UpContext(ctx, db, ".")
	if err != nil {
		return err
	}
	return nil
}
