package migrations

import (
	"context"
	"database/sql"

	"github.com/pressly/goose/v3"
)

func init() {
	goose.AddNamedMigrationContext("202610250003_add_download_job_cleanup_status.go", addDownloadJobCleanupStatus, irreversibleMigration)
}

func addDownloadJobCleanupStatus(ctx context.Context, tx *sql.Tx) error {
	return addColumnIfMissing(ctx, tx, "download_job", "cleanup_status", `text NOT NULL DEFAULT ""`)
}
