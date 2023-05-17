package database

import (
	"database/sql"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

func Open() (db *sql.DB, err error) {
	db, err = sql.Open("sqlite3", "qsurvey.sqlite") // TODO pass in db name as parameter
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
