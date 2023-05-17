package database

import (
	"database/sql"
	"embed"
	"errors"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/sqlite3"
	"github.com/golang-migrate/migrate/v4/source/iofs"
)

//go:embed migrations
var dbMigrations embed.FS

func migrateDB(db *sql.DB) error {
	src, err := iofs.New(dbMigrations, "migrations")
	if err != nil {
		return err
	}

	dst, err := sqlite3.WithInstance(db, &sqlite3.Config{})
	if err != nil {
		return err
	}

	migrator, err := migrate.NewWithInstance("iofs", src, "sqlite3", dst)
	if err != nil {
		return err
	}

	err = migrator.Up()
	switch {
	case errors.Is(err, migrate.ErrNoChange):
		// db already up to date
		break
	case err != nil:
		return err
	}
	return nil
}
