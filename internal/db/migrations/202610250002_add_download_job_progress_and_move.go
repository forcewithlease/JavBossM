package migrations

import (
	"context"
	"database/sql"

	"github.com/pressly/goose/v3"
)

func init() {
	goose.AddNamedMigrationContext("202610250002_add_download_job_progress_and_move.go", addDownloadJobProgressAndMove, irreversibleMigration)
}

func addDownloadJobProgressAndMove(ctx context.Context, tx *sql.Tx) error {
	for _, column := range []struct {
		name string
		decl string
	}{
		{"download_speed", "integer NOT NULL DEFAULT 0"},
		{"moved_path", `text NOT NULL DEFAULT ""`},
		{"move_status", `text NOT NULL DEFAULT ""`},
		{"move_error", `text NOT NULL DEFAULT ""`},
	} {
		if err := addColumnIfMissing(ctx, tx, "download_job", column.name, column.decl); err != nil {
			return err
		}
	}
	return nil
}
