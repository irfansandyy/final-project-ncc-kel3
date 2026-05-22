package db

import (
	"database/sql"

	_ "github.com/jackc/pgx/v5/stdlib"
)

func Connect(dsn string) (*sql.DB, error) {
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, err
	}

	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(25)

	if err := db.Ping(); err != nil {
		return nil, err
	}

	return db, nil
}
