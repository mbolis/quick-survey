package database

import (
	"database/sql"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"github.com/mbolis/quick-survey/config"
)

func Open(cfg config.Config) (db *sql.DB, err error) {
	db, err = sql.Open("sqlite3", cfg.DBUrl)
	if err != nil {
		return
	}

	_, err = db.Exec("PRAGMA foreign_keys = ON")
	if err != nil {
		db.Close()
		return
	}

	// db tuning options
	db.SetMaxOpenConns(20)
	db.SetMaxIdleConns(10)
	db.SetConnMaxIdleTime(5 * time.Minute)
	db.SetConnMaxLifetime(2 * time.Hour)

	err = migrateDB(db)
	if err != nil {
		db.Close()
		return
	}

	return
}
