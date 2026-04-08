// Package sqlite provides a YAML-defined SQLite component for Origami circuits.
//
// The component replaces hand-written SQL DDL strings with declarative YAML schema
// definitions. It provides:
//
//   - Schema parser: YAML table/column/index definitions → Go types
//   - DDL generator: Go types → CREATE TABLE / CREATE INDEX SQL
//   - Migration engine: versioned YAML operations (create_table, add_column,
//     rename_table, drop_table, raw_sql) applied in order inside a transaction
//   - DB manager: Open (file-backed) and OpenMemory (in-memory for tests)
//   - CRUD helpers: parameterized Insert, QueryRows, QueryOne, Update
//
// Design follows the Ansible sqlite_utils collection pattern:
// generic operations in the framework, domain schema/data in the consumer.
package sqlite
