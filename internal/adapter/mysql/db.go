package mysql

import (
	"database/sql"
	"fmt"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/golang-migrate/migrate/v4"
	mysqlmigrate "github.com/golang-migrate/migrate/v4/database/mysql"
	"github.com/golang-migrate/migrate/v4/source/iofs"

	"github.com/incredible-zetta/crm/internal/port"
	"github.com/incredible-zetta/crm/migrations"
)

// Open opens a database connection, sets pool limits, and pings the database.
func Open(dsn string) (*sql.DB, error) {
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, fmt.Errorf("open mysql: %w", err)
	}

	db.SetMaxOpenConns(20)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)

	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}

	return db, nil
}

// Migrate runs embedded SQL migrations up to the latest version.
func Migrate(db *sql.DB) error {
	srcDriver, err := iofs.New(migrations.FS, ".")
	if err != nil {
		return fmt.Errorf("migrate iofs source: %w", err)
	}

	dbDriver, err := mysqlmigrate.WithInstance(db, &mysqlmigrate.Config{})
	if err != nil {
		return fmt.Errorf("migrate mysql driver: %w", err)
	}

	m, err := migrate.NewWithInstance("iofs", srcDriver, "mysql", dbDriver)
	if err != nil {
		return fmt.Errorf("migrate init: %w", err)
	}

	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		return fmt.Errorf("migrate up: %w", err)
	}

	return nil
}

// Store holds the DB connection and returns repo implementations.
type Store struct {
	db *sql.DB
}

// New creates a new Store instance.
func New(db *sql.DB) *Store {
	return &Store{db: db}
}

// Contacts returns a ContactRepo implementation.
func (s *Store) Contacts() port.ContactRepo {
	return &contactRepo{db: s.db}
}

// Campaigns returns a CampaignRepo implementation.
func (s *Store) Campaigns() port.CampaignRepo {
	return &campaignRepo{db: s.db}
}

// Templates returns a TemplateRepo implementation.
func (s *Store) Templates() port.TemplateRepo {
	return &templateRepo{db: s.db}
}

// Tasks returns a TaskRepo implementation.
func (s *Store) Tasks() port.TaskRepo {
	return &taskRepo{db: s.db}
}

// Events returns an EventRepo implementation.
func (s *Store) Events() port.EventRepo {
	return &eventRepo{db: s.db}
}

// Tracking returns a TrackingRepo implementation.
func (s *Store) Tracking() port.TrackingRepo {
	return &trackingRepo{db: s.db}
}

// Exports returns an ExportRepo implementation.
func (s *Store) Exports() port.ExportRepo {
	return &exportRepo{db: s.db}
}

// Inbox returns an InboxRepo implementation.
func (s *Store) Inbox() port.InboxRepo {
	return &inboxRepo{db: s.db}
}

// WhatsApp returns a WAMessageRepo implementation.
func (s *Store) WhatsApp() port.WAMessageRepo {
	return &waMessageRepo{db: s.db}
}

// Threads returns a ThreadsRepo implementation.
func (s *Store) Threads() port.ThreadsRepo {
	return &threadsRepo{db: s.db}
}
