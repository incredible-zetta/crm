package db

import (
	"database/sql"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/golang-migrate/migrate/v4"
	mysqlmigrate "github.com/golang-migrate/migrate/v4/database/mysql"
	"github.com/golang-migrate/migrate/v4/source/iofs"

	"github.com/cipta/crm-for-aiagents/migrations"
)

// Open opens a database connection, sets pool limits, and pings the database.
func Open(dsn string) (*sql.DB, error) {
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, err
	}

	db.SetMaxOpenConns(20)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)

	if err := db.Ping(); err != nil {
		db.Close()
		return nil, err
	}

	return db, nil
}

// Migrate runs embedded SQL migrations up to the latest version.
func Migrate(db *sql.DB) error {
	srcDriver, err := iofs.New(migrations.FS, ".")
	if err != nil {
		return err
	}

	dbDriver, err := mysqlmigrate.WithInstance(db, &mysqlmigrate.Config{})
	if err != nil {
		return err
	}

	m, err := migrate.NewWithInstance("iofs", srcDriver, "mysql", dbDriver)
	if err != nil {
		return err
	}

	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		return err
	}

	return nil
}
