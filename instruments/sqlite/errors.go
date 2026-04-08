package sqlite

import "errors"

var (
	// ErrTableNameRequired is returned when a table name is empty.
	ErrTableNameRequired = errors.New("table name is required")

	// ErrColumnsRequired is returned when no columns are provided.
	ErrColumnsRequired = errors.New("columns are required")

	// ErrColumnValuesMismatch is returned when columns and values counts differ.
	ErrColumnValuesMismatch = errors.New("columns/values count mismatch")

	// ErrSetColumnsRequired is returned when no SET columns are provided for update.
	ErrSetColumnsRequired = errors.New("set columns are required")

	// ErrUnknownTable is returned when referencing a table not in the schema.
	ErrUnknownTable = errors.New("unknown table")

	// ErrNoColumns is returned when a table has no columns provided.
	ErrNoColumns = errors.New("no columns provided")

	// ErrConditionsRequired is returned when no conditions are given for get_by.
	ErrConditionsRequired = errors.New("conditions required")

	// ErrNotFound is returned when an entity is not found by ID.
	ErrNotFound = errors.New("not found")

	// ErrMigrationVersionRequired is returned when migration versions are missing.
	ErrMigrationVersionRequired = errors.New("migration must specify from and to versions")

	// ErrMigrationVersionOrder is returned when To <= From.
	ErrMigrationVersionOrder = errors.New("migration to must be greater than from")

	// ErrMigrationNoOps is returned when a migration has no operations.
	ErrMigrationNoOps = errors.New("migration has no operations")

	// ErrColumnMustBeMapping is returned when a column YAML node is not a mapping.
	ErrColumnMustBeMapping = errors.New("column must be a mapping")

	// ErrColumnTypeRequired is returned when a column type is missing in shorthand.
	ErrColumnTypeRequired = errors.New("type is required in shorthand")

	// ErrReferenceTableRequired is returned when a reference modifier has no table.
	ErrReferenceTableRequired = errors.New("requires a table name")

	// ErrUnknownModifier is returned for an unknown column modifier.
	ErrUnknownModifier = errors.New("unknown modifier")

	// ErrTableNoColumns is returned when a table definition has no columns.
	ErrTableNoColumns = errors.New("table has no columns")

	// ErrSchemaVersionRequired is returned when the schema version is missing.
	ErrSchemaVersionRequired = errors.New("schema version is required")

	// ErrDuplicateTable is returned for duplicate table names.
	ErrDuplicateTable = errors.New("duplicate table")

	// ErrColumnNameRequired is returned when a column name is empty.
	ErrColumnNameRequired = errors.New("column name is required")

	// ErrDuplicateColumn is returned for duplicate column names in a table.
	ErrDuplicateColumn = errors.New("duplicate column")

	// ErrUnknownColumnRef is returned when a constraint references an unknown column.
	ErrUnknownColumnRef = errors.New("references unknown column")

	// ErrIndexNameRequired is returned when an index name is empty.
	ErrIndexNameRequired = errors.New("index name is required")

	// ErrIndexUnknownTable is returned when an index references an unknown table.
	ErrIndexUnknownTable = errors.New("index references unknown table")
)
