package sqlite

import (
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

// Schema is the top-level YAML definition for a SQLite database.
type Schema struct {
	Version int     `yaml:"version"`
	Tables  []Table `yaml:"tables"`
	Indexes []Index `yaml:"indexes,omitempty"`
}

// Table defines a single SQLite table.
type Table struct {
	Name        string       `yaml:"name"`
	Columns     []Column     `yaml:"columns"`
	ForeignKeys []ForeignKey `yaml:"foreign_keys,omitempty"`
	Unique      [][]string   `yaml:"unique,omitempty"`
}

// Column defines a single column in a table.
type Column struct {
	Name          string `yaml:"name"`
	Type          string `yaml:"type"`
	PrimaryKey    bool   `yaml:"primary_key,omitempty"`
	Autoincrement bool   `yaml:"autoincrement,omitempty"`
	NotNull       bool   `yaml:"not_null,omitempty"`
	Unique        bool   `yaml:"unique,omitempty"`
	Default       string `yaml:"default,omitempty"`
	References    string `yaml:"references,omitempty"`
}

// ForeignKey defines a table-level foreign key constraint.
type ForeignKey struct {
	Columns    []string `yaml:"columns"`
	References string   `yaml:"references"`
}

// Index defines a database index.
type Index struct {
	Name    string   `yaml:"name"`
	Table   string   `yaml:"table"`
	Columns []string `yaml:"columns"`
	Unique  bool     `yaml:"unique,omitempty"`
}

// rawSchema is the parse-time representation that supports shorthand features.
// It is normalized into a Schema before validation.
// Note: Envelope is not embedded here because rawSchema.Version (int) conflicts
// with Envelope.Version (string). Use framework.ParseEnvelope separately.
type rawSchema struct {
	Version int        `yaml:"version"`
	Tables  []rawTable `yaml:"tables"`
	Indexes []Index    `yaml:"indexes,omitempty"`
}

// rawTable supports auto_id and table-local indexes during parsing.
type rawTable struct {
	Name        string       `yaml:"name"`
	AutoID      *bool        `yaml:"auto_id,omitempty"`
	Columns     []Column     `yaml:"columns"`
	ForeignKeys []ForeignKey `yaml:"foreign_keys,omitempty"`
	Unique      [][]string   `yaml:"unique,omitempty"`
	Indexes     [][]string   `yaml:"indexes,omitempty"`
}

var knownColumnTypes = map[string]bool{
	"integer": true, "int": true,
	"text": true, "varchar": true,
	"real": true, "float": true, "double": true,
	"blob": true, "boolean": true,
}

// UnmarshalYAML supports both verbose and shorthand column definitions.
//
// Verbose (existing):
//
//	name: email
//	type: text
//	not_null: true
//
// Shorthand:
//
//	email: text not_null unique -> users
//
// Modifiers: not_null, unique, pk, auto, default=VALUE, -> TABLE
func (c *Column) UnmarshalYAML(value *yaml.Node) error {
	if value.Kind != yaml.MappingNode {
		return fmt.Errorf("column must be a mapping")
	}
	if hasYAMLKey(value, "type") {
		type plain Column
		var p plain
		if err := value.Decode(&p); err != nil {
			return err
		}
		*c = Column(p)
		return nil
	}
	if len(value.Content) == 2 {
		key := value.Content[0]
		val := value.Content[1]
		if key.Kind == yaml.ScalarNode && val.Kind == yaml.ScalarNode {
			first, _, _ := strings.Cut(val.Value, " ")
			if knownColumnTypes[strings.ToLower(first)] {
				return c.parseShorthand(key.Value, val.Value)
			}
		}
	}
	type plain Column
	var p plain
	if err := value.Decode(&p); err != nil {
		return err
	}
	*c = Column(p)
	return nil
}

func (c *Column) parseShorthand(name, spec string) error {
	c.Name = name
	tokens := strings.Fields(spec)
	if len(tokens) == 0 {
		return fmt.Errorf("column %q: type is required in shorthand", name)
	}
	c.Type = tokens[0]
	for i := 1; i < len(tokens); i++ {
		tok := tokens[i]
		switch {
		case tok == "not_null":
			c.NotNull = true
		case tok == "unique":
			c.Unique = true
		case tok == "pk":
			c.PrimaryKey = true
		case tok == "auto":
			c.Autoincrement = true
		case strings.HasPrefix(tok, "default="):
			c.Default = strings.TrimPrefix(tok, "default=")
		case tok == "->" || tok == "references":
			if i+1 >= len(tokens) {
				return fmt.Errorf("column %q: %s requires a table name", name, tok)
			}
			i++
			ref := tokens[i]
			if !strings.Contains(ref, "(") {
				ref += "(id)"
			}
			c.References = ref
		default:
			return fmt.Errorf("column %q: unknown modifier %q", name, tok)
		}
	}
	return nil
}

func hasYAMLKey(node *yaml.Node, key string) bool {
	for i := 0; i < len(node.Content)-1; i += 2 {
		if node.Content[i].Kind == yaml.ScalarNode && node.Content[i].Value == key {
			return true
		}
	}
	return false
}

// ParseSchemaFile reads and parses a YAML schema from a file path.
func ParseSchemaFile(path string) (*Schema, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read schema file %s: %w", path, err)
	}
	return ParseSchema(data)
}

// ParseSchema parses a YAML schema from bytes. It supports both verbose
// and shorthand column definitions, implicit id columns, and table-local indexes.
func ParseSchema(data []byte) (*Schema, error) {
	var raw rawSchema
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("parse schema YAML: %w", err)
	}
	s, err := raw.normalize()
	if err != nil {
		return nil, err
	}
	if err := s.Validate(); err != nil {
		return nil, err
	}
	return s, nil
}

func (raw *rawSchema) normalize() (*Schema, error) {
	s := &Schema{
		Version: raw.Version,
		Indexes: append([]Index{}, raw.Indexes...),
	}
	for _, rt := range raw.Tables {
		if len(rt.Columns) == 0 {
			return nil, fmt.Errorf("table %q has no columns", rt.Name)
		}
		t := Table{
			Name:        rt.Name,
			Columns:     rt.Columns,
			ForeignKeys: rt.ForeignKeys,
			Unique:      rt.Unique,
		}
		if rt.shouldAutoID() {
			idCol := Column{
				Name: "id", Type: colTypeInteger,
				PrimaryKey: true, Autoincrement: true,
			}
			t.Columns = append([]Column{idCol}, t.Columns...)
		}
		for _, cols := range rt.Indexes {
			s.Indexes = append(s.Indexes, Index{
				Name:    fmt.Sprintf("idx_%s_%s", rt.Name, strings.Join(cols, "_")),
				Table:   rt.Name,
				Columns: cols,
			})
		}
		s.Tables = append(s.Tables, t)
	}
	return s, nil
}

func (rt *rawTable) shouldAutoID() bool {
	if rt.AutoID != nil {
		return *rt.AutoID
	}
	for _, c := range rt.Columns {
		if c.Name == "id" {
			return false
		}
	}
	return true
}

// Validate checks the schema for structural errors.
func (s *Schema) Validate() error {
	if s.Version == 0 {
		return fmt.Errorf("schema version is required")
	}
	tables := make(map[string]bool, len(s.Tables))
	for _, t := range s.Tables {
		if t.Name == "" {
			return fmt.Errorf("table name is required")
		}
		if tables[t.Name] {
			return fmt.Errorf("duplicate table %q", t.Name)
		}
		tables[t.Name] = true
		if len(t.Columns) == 0 {
			return fmt.Errorf("table %q has no columns", t.Name)
		}
		cols := make(map[string]bool, len(t.Columns))
		for _, c := range t.Columns {
			if c.Name == "" {
				return fmt.Errorf("table %q: column name is required", t.Name)
			}
			if cols[c.Name] {
				return fmt.Errorf("table %q: duplicate column %q", t.Name, c.Name)
			}
			cols[c.Name] = true
			if c.Type == "" {
				return fmt.Errorf("table %q: column %q type is required", t.Name, c.Name)
			}
		}
		for _, uc := range t.Unique {
			for _, col := range uc {
				if !cols[col] {
					return fmt.Errorf("table %q: unique constraint references unknown column %q", t.Name, col)
				}
			}
		}
	}
	for _, idx := range s.Indexes {
		if idx.Name == "" {
			return fmt.Errorf("index name is required")
		}
		if !tables[idx.Table] {
			return fmt.Errorf("index %q references unknown table %q", idx.Name, idx.Table)
		}
	}
	return nil
}

// GenerateDDL produces CREATE TABLE and CREATE INDEX statements from the schema.
func (s *Schema) GenerateDDL() string {
	var b strings.Builder
	for i := range s.Tables {
		if i > 0 {
			b.WriteString("\n")
		}
		b.WriteString(generateTableDDL(&s.Tables[i]))
	}
	for _, idx := range s.Indexes {
		b.WriteString("\n")
		b.WriteString(generateIndexDDL(idx))
	}
	return b.String()
}

func generateTableDDL(t *Table) string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("CREATE TABLE IF NOT EXISTS %s (\n", t.Name))

	lines := make([]string, 0, len(t.Columns)+len(t.ForeignKeys)+len(t.Unique))
	for _, c := range t.Columns {
		lines = append(lines, "\t"+generateColumnDDL(c))
	}
	for _, fk := range t.ForeignKeys {
		lines = append(lines, fmt.Sprintf("\tFOREIGN KEY (%s) REFERENCES %s",
			strings.Join(fk.Columns, ", "), fk.References))
	}
	for _, uc := range t.Unique {
		lines = append(lines, fmt.Sprintf("\tUNIQUE(%s)", strings.Join(uc, ", ")))
	}

	b.WriteString(strings.Join(lines, ",\n"))
	b.WriteString("\n);\n")
	return b.String()
}

func generateColumnDDL(c Column) string {
	var parts []string
	parts = append(parts, c.Name, strings.ToUpper(c.Type))
	if c.PrimaryKey {
		parts = append(parts, "PRIMARY KEY")
	}
	if c.Autoincrement {
		parts = append(parts, "AUTOINCREMENT")
	}
	if c.NotNull {
		parts = append(parts, "NOT NULL")
	}
	if c.Unique {
		parts = append(parts, "UNIQUE")
	}
	if c.Default != "" {
		parts = append(parts, "DEFAULT", c.Default)
	}
	if c.References != "" {
		parts = append(parts, "REFERENCES", c.References)
	}
	return strings.Join(parts, " ")
}

func generateIndexDDL(idx Index) string {
	u := ""
	if idx.Unique {
		u = "UNIQUE "
	}
	return fmt.Sprintf("CREATE %sINDEX IF NOT EXISTS %s ON %s(%s);\n",
		u, idx.Name, idx.Table, strings.Join(idx.Columns, ", "))
}
