package migrations

import (
	"context"
	"database/sql"

	"github.com/pressly/goose/v3"
)

func init() {
	goose.AddNamedMigrationContext("202610250001_add_downloader_provider_username.go", addDownloaderProviderUsername, irreversibleMigration)
}

// addDownloaderProviderUsername rebuilds the table so the new column keeps the
// same position and default as the GORM model instead of being appended.
func addDownloaderProviderUsername(ctx context.Context, tx *sql.Tx) error {
	exists, err := columnExists(ctx, tx, "downloader_provider_settings", "username")
	if err != nil {
		return err
	}
	if exists {
		return nil
	}
	return execStatements(ctx, tx,
		`ALTER TABLE "downloader_provider_settings" RENAME TO "downloader_provider_settings_old"`,
		`CREATE TABLE "downloader_provider_settings" (
			provider text PRIMARY KEY,
			address text NOT NULL DEFAULT "",
			username text NOT NULL DEFAULT "",
			api_token text NOT NULL DEFAULT "",
			remote_folder text NOT NULL DEFAULT "",
			created_at datetime,
			updated_at datetime
		)`,
		`INSERT INTO "downloader_provider_settings" (
			provider, address, username, api_token, remote_folder, created_at, updated_at
		) SELECT provider, address, '', api_token, remote_folder, created_at, updated_at
		FROM "downloader_provider_settings_old"`,
		`DROP TABLE "downloader_provider_settings_old"`,
	)
}
