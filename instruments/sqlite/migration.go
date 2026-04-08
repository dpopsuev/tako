package sqlite

import (
	"database/sql"
	"fmt"
	"os"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// Migration defines a versioned schema migration as YAML.
type Migration struct {
	From       int         `yaml:"from"`
	To         int         `yaml:"to"`
	Operations []Operation `yaml:"operations"`
}

// Operation is a single migration step.
type Operation struct {
	// Exactly one of these should be set per operation.
	CreateTable *Table       `yaml:"create_table,omitempty"`
	AddColumn   *AddColumn   `yaml:"add_column,omitempty"`
	RenameTable *RenameTable `yaml:"rename_table,omitempty"`
	DropTable   *DropTable   `yaml:"drop_table,omitempty"`
	CreateIndex *Index       `yaml:"create_index,omitempty"`
	RawSQL      string       `yaml:"raw_sql,omitempty"`
}

// AddColumn adds a column to an existing table.
type AddColumn struct {
	Table  string `yaml:"table"`
	Column Column `yaml:"column"`
}

// RenameTable renames a table.
type RenameTable struct {
	From string `yaml:"from"`
	To   string `yaml:"to"`
}

// DropTable drops a table.
type DropTable struct {
	Name string `yaml:"name"`
}

// ParseMigrationFile reads and parses a YAML migration from a file.
func ParseMigrationFile(path string) (*Migration, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read migration file %s: %w", path, err)
	}
	return ParseMigration(data)
}

// ParseMigration parses a YAML migration from bytes.
func ParseMigration(data []byte) (*Migration, error) {
	var m Migration
	if err := yaml.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("parse migration YAML: %w", err)
	}
	if m.From == 0 || m.To == 0 {
		return nil, ErrMigrationVersionRequired
	}
	if m.To <= m.From {
		return nil, fmt.Errorf("%w (%d vs %d)", ErrMigrationVersionOrder, m.To, m.From)
	}
	if len(m.Operations) == 0 {
		return nil, ErrMigrationNoOps
	}
	return &m, nil
}

// GenerateSQL produces the SQL statements for this migration.
func (m *Migration) GenerateSQL() string {
	var b strings.Builder
	for _, op := range m.Operations {
		b.WriteString(generateOpSQL(op))
	}
	return b.String()
}

func generateOpSQL(op Operation) string {
	switch {
	case op.CreateTable != nil:
		return generateTableDDL(op.CreateTable)
	case op.AddColumn != nil:
		ac := op.AddColumn
		return fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s;\n",
			ac.Table, generateColumnDDL(ac.Column))
	case op.RenameTable != nil:
		return fmt.Sprintf("ALTER TABLE %s RENAME TO %s;\n",
			op.RenameTable.From, op.RenameTable.To)
	case op.DropTable != nil:
		return fmt.Sprintf("DROP TABLE IF EXISTS %s;\n", op.DropTable.Name)
	case op.CreateIndex != nil:
		return generateIndexDDL(*op.CreateIndex)
	case op.RawSQL != "":
		s := strings.TrimSpace(op.RawSQL)
		if !strings.HasSuffix(s, ";") {
			s += ";"
		}
		return s + "\n"
	default:
		return ""
	}
}

// RunMigrations executes a sequence of migrations against a database,
// starting from the current version and applying each in order.
// Migrations are sorted by From version and applied inside a transaction.
func RunMigrations(db *sql.DB, migrations []*Migration) error {
	currentVersion, err := getSchemaVersion(db)
	if err != nil {
		return err
	}

	sort.Slice(migrations, func(i, j int) bool {
		return migrations[i].From < migrations[j].From
	})

	for _, m := range migrations {
		if m.From != currentVersion {
			continue
		}
		if err := applyMigration(db, m); err != nil {
			return fmt.Errorf("migration v%d→v%d: %w", m.From, m.To, err)
		}
		currentVersion = m.To
	}
	return nil
}

func applyMigration(db *sql.DB, m *Migration) error {
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.Exec("PRAGMA foreign_keys = OFF"); err != nil {
		return fmt.Errorf("disable foreign keys: %w", err)
	}

	sql := m.GenerateSQL()
	if _, err := tx.Exec(sql); err != nil {
		return fmt.Errorf("execute migration SQL: %w", err)
	}

	if _, err := tx.Exec("UPDATE schema_version SET version = ?", m.To); err != nil {
		return fmt.Errorf("update schema version: %w", err)
	}

	return tx.Commit()
}
