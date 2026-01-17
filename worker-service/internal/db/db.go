package db

import (
	"errors"
	"fmt"
	"log"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
	"github.com/sirupsen/logrus"
)

func InitDB(conn string) (*sqlx.DB, error) {
	fmt.Println("connecting to DB")

	DB, err := sqlx.Connect("postgres", conn)
	if err != nil {
		return nil, err
	}

	if err := DB.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %v", err)
	}

	Migrations(DB)

	log.Println("Database connection established")

	return DB, nil
}

func Migrations(db *sqlx.DB) {
	driver, err := postgres.WithInstance(db.DB, &postgres.Config{})
	if err != nil {
		logrus.Fatalf("MIGRATIONS: failed to create postgres driver: %v", err)
	}

	m, err := migrate.NewWithDatabaseInstance(
		"file://internal/db/migrations",
		"postgres",
		driver,
	)
	if err != nil {
		logrus.Fatalf("MIGRATIONS: failed to initialize migrator: %v", err)
	}
	err = m.Up()
	if err != nil && !errors.Is(migrate.ErrNoChange, err) {
		logrus.Fatalf("MIGRATIONS: failed applied migrations: %v", err)
	}
	logrus.Info("MIGRATIONS: database is up to date")
}
