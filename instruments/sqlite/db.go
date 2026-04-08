package sqlite

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite" // register sqlite3 driver
)

// DB wraps a sql.DB with schema-aware lifecycle management.
type DB struct {
	*sql.DB
	schema *Schema
}

// Open opens or creates a SQLite database at the given path.
// If schema is non-nil, it applies the schema (fresh install) or validates the
// current version and runs migrations.
func Open(path string, schema *Schema) (*DB, error) {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("create db dir: %w", err)
	}
	sqlDB, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open sqlite %s: %w", path, err)
	}
	if err := sqlDB.Ping(); err != nil {
		sqlDB.Close()
		return nil, fmt.Errorf("ping sqlite: %w", err)
	}
	d := &DB{DB: sqlDB, schema: schema}
	if schema != nil {
		if err := d.applySchema(); err != nil {
			sqlDB.Close()
			return nil, err
		}
	}
	return d, nil
}

// OpenMemory opens an in-memory SQLite database with the given schema applied.
// Useful for testing — no file I/O, fast teardown.
func OpenMemory(schema *Schema) (*DB, error) {
	sqlDB, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		return nil, fmt.Errorf("open memory sqlite: %w", err)
	}
	d := &DB{DB: sqlDB, schema: schema}
	if schema != nil {
		if err := d.applySchema(); err != nil {
			sqlDB.Close()
			return nil, err
		}
	}
	return d, nil
}

// Schema returns the loaded schema definition, or nil if none was provided.
func (d *DB) Schema() *Schema {
	return d.schema
}

// Migrate runs a set of migrations against this database.
func (d *DB) Migrate(migrations []*Migration) error {
	return RunMigrations(d.DB, migrations)
}

func (d *DB) applySchema() error {
	version, err := getSchemaVersion(d.DB)
	if err != nil {
		return err
	}

	if version > 0 {
		return nil
	}

	ddl := d.schema.GenerateDDL()
	if _, err := d.Exec(ddl); err != nil {
		return fmt.Errorf("apply schema DDL: %w", err)
	}
	if _, err := d.Exec(
		"CREATE TABLE IF NOT EXISTS schema_version (version INTEGER NOT NULL)"); err != nil {
		return fmt.Errorf("create schema_version: %w", err)
	}
	if _, err := d.Exec(
		"INSERT INTO schema_version(version) VALUES(?)", d.schema.Version); err != nil {
		return fmt.Errorf("set schema version: %w", err)
	}
	return nil
}

// getSchemaVersion reads the current schema version from the database.
// Returns 0 if the schema_version table does not exist (fresh DB).
func getSchemaVersion(db *sql.DB) (int, error) {
	var count int
	err := db.QueryRow(
		"SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='schema_version'",
	).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("check schema_version: %w", err)
	}
	if count == 0 {
		return 0, nil
	}
	var v int
	err = db.QueryRow("SELECT version FROM schema_version LIMIT 1").Scan(&v)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("read schema version: %w", err)
	}
	return v, nil
}
