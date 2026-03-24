package curate

// FieldRequirement defines whether a field is required or optional.
type FieldRequirement string

const (
	Required FieldRequirement = "required"
	Optional FieldRequirement = "optional"
)

// FieldSpec describes one field in a schema: its name, requirement level,
// and an optional validation function.
type FieldSpec struct {
	Name        string          `json:"name"`
	Requirement FieldRequirement `json:"requirement"`
	Description string          `json:"description,omitempty"`
	Validate    func(value any) bool `json:"-"`
}

// Schema defines the expected structure for records in a dataset.
// It drives completeness checking and promotion gates.
type Schema struct {
	Name   string      `json:"name"`
	Fields []FieldSpec `json:"fields"`
}

// RequiredFields returns only the fields marked as Required.
func (s Schema) RequiredFields() []FieldSpec {
	var out []FieldSpec
	for _, f := range s.Fields {
		if f.Requirement == Required {
			out = append(out, f)
		}
	}
	return out
}

// CompletenessResult scores a record against a schema.
type CompletenessResult struct {
	RecordID   string   `json:"record_id"`
	Score      float64  `json:"score"`
	Present    []string `json:"present"`
	Missing    []string `json:"missing"`
	Invalid    []string `json:"invalid,omitempty"`
	Promotable bool     `json:"promotable"`
}

// CheckCompleteness evaluates a Record against a Schema.
// A record is promotable when all required fields are present and valid.
func CheckCompleteness(r Record, s Schema) CompletenessResult {
	result := CompletenessResult{
		RecordID: r.ID,
	}

	total := 0
	present := 0

	for _, spec := range s.Fields {
		if spec.Requirement != Required {
			continue
		}
		total++

		f, has := r.Get(spec.Name)
		if !has || f.Value == nil {
			result.Missing = append(result.Missing, spec.Name)
			continue
		}

		if spec.Validate != nil && !spec.Validate(f.Value) {
			result.Invalid = append(result.Invalid, spec.Name)
			continue
		}

		present++
		result.Present = append(result.Present, spec.Name)
	}

	if total > 0 {
		result.Score = float64(present) / float64(total)
	}
	result.Promotable = len(result.Missing) == 0 && len(result.Invalid) == 0

	return result
}
