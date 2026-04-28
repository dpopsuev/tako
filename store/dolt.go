package store

import (
	"fmt"
	"os"
	"path/filepath"

	_ "github.com/dolthub/driver" // register dolt sql driver
	"github.com/jmoiron/sqlx"
)

// DB wraps a sqlx.DB connected to an embedded Dolt instance.
type DB struct {
	*sqlx.DB
	dir string
}

// Open creates or opens an embedded Dolt database at the given directory.
func Open(dir string) (*DB, error) {
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return nil, fmt.Errorf("store: mkdir %s: %w", dir, err)
	}

	absDir, err := filepath.Abs(dir)
	if err != nil {
		return nil, fmt.Errorf("store: abs path: %w", err)
	}

	dsn := fmt.Sprintf("file://%s?commitname=tako&commitemail=tako@local&database=tako", absDir)
	db, err := sqlx.Open("dolt", dsn)
	if err != nil {
		return nil, fmt.Errorf("store: open dolt: %w", err)
	}

	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("store: ping dolt: %w", err)
	}

	return &DB{DB: db, dir: absDir}, nil
}

// Dir returns the filesystem path of this Dolt database.
func (d *DB) Dir() string {
	return d.dir
}

// Migrate creates the database and core tables if they don't exist.
func (d *DB) Migrate() error {
	if _, err := d.Exec("CREATE DATABASE IF NOT EXISTS tako"); err != nil {
		return fmt.Errorf("store: create database: %w", err)
	}
	if _, err := d.Exec("USE tako"); err != nil {
		return fmt.Errorf("store: use database: %w", err)
	}
	for _, ddl := range migrations {
		if _, err := d.Exec(ddl); err != nil {
			return fmt.Errorf("store: migrate: %w", err)
		}
	}
	return nil
}

var migrations = []string{
	`CREATE TABLE IF NOT EXISTS envelopes (
		id         VARCHAR(255) PRIMARY KEY,
		shelf_name VARCHAR(255) NOT NULL,
		origin     VARCHAR(255) NOT NULL,
		payload    LONGBLOB,
		labels     JSON,
		hash       VARCHAR(64),
		state      VARCHAR(32) NOT NULL DEFAULT 'UNCLAIMED',
		claimed_by VARCHAR(255),
		claimed_at DATETIME,
		expires_at DATETIME,
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	)`,
	`CREATE TABLE IF NOT EXISTS knowledge_nodes (
		id         VARCHAR(255) PRIMARY KEY,
		content    TEXT,
		tier       INT NOT NULL DEFAULT 0,
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	)`,
	`CREATE TABLE IF NOT EXISTS knowledge_edges (
		from_id    VARCHAR(255) NOT NULL,
		to_id      VARCHAR(255) NOT NULL,
		relation   VARCHAR(255),
		weight     DOUBLE NOT NULL DEFAULT 1.0,
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		PRIMARY KEY (from_id, to_id)
	)`,
	`CREATE TABLE IF NOT EXISTS ergograph_records (
		id        INT AUTO_INCREMENT PRIMARY KEY,
		identity  VARCHAR(255),
		action    VARCHAR(255) NOT NULL,
		timestamp DATETIME NOT NULL,
		sequence  BIGINT NOT NULL,
		labels    JSON,
		payload   BLOB,
		hash      VARCHAR(64),
		prev_hash VARCHAR(64)
	)`,
}
