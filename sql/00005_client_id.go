package migrations

import (
	"context"
	"database/sql"
	"errors"

	"github.com/openkcm/common-sdk/pkg/commoncfg"
	"github.com/pressly/goose/v3"

	slogctx "github.com/veqryn/slog-context"

	"github.com/openkcm/session-manager/internal/config"
)

func init() {
	goose.AddMigrationContext(Up00005, Down00005)
}

func Up00005(ctx context.Context, tx *sql.Tx) error {
	clientID, err := readClientIDfromConfig()
	if err != nil {
		return err
	}
	slogctx.Debug(ctx, "Updating trust table with client_id", "client_id", clientID)
	_, err = tx.ExecContext(ctx, "UPDATE trust SET client_id=$1 WHERE client_id IS NULL or client_id='';", clientID)
	return err
}

func Down00005(ctx context.Context, tx *sql.Tx) error {
	return nil
}

func readClientIDfromConfig() (string, error) {
	// Load the config which contains the client_id
	cfg := &config.Config{}
	loader := commoncfg.NewLoader(cfg, commoncfg.WithPaths(
		"/etc/session-manager",
		"$HOME/.session-manager",
		".",
	))
	if err := loader.LoadConfig(); err != nil {
		return "", err
	}

	// Read the client_id from the config
	//nolint:staticcheck
	clientID := cfg.SessionManager.ClientAuth.ClientID
	if clientID == "" {
		return "", errors.New("client_id is not set in the config")
	}

	return clientID, nil
}
