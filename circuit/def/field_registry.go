package def

// FieldDef describes a single YAML field in the DSL.
// Every exported struct field with a yaml tag must have a FieldDef entry.
type FieldDef struct {
	// Required means the field must be non-zero for valid circuits.
	Required bool
	// ValidValues enumerates the accepted enum values for this field.
	// nil means any value is accepted (no enum constraint).
	ValidValues []string
}

// FieldRegistry maps yaml tag names to their definitions.
// One registry per DSL struct. The trap test ensures every YAML field
// on the struct has an entry — adding a field without registering it
// causes a test failure.
type FieldRegistry map[string]FieldDef

// Has returns true if the field is registered.
func (r FieldRegistry) Has(yamlTag string) bool {
	_, ok := r[yamlTag]
	return ok
}

// RequiredFields returns all field names marked as required.
func (r FieldRegistry) RequiredFields() []string {
	var result []string
	for name, def := range r {
		if def.Required {
			result = append(result, name)
		}
	}
	return result
}

// CircuitFields registers every yaml field on CircuitDef.
var CircuitFields = FieldRegistry{
	"circuit":      {Required: true},
	"description":  {},
	"import":       {},
	"topology":     {},
	"handler_type": {ValidValues: ValidHandlerTypes},
	"timeout":      {},
	"imports":      {},
	"vars":         {},
	"extractors":   {},
	"ports":        {},
	"wiring":       {},
	"zones":        {},
	"nodes":        {Required: true},
	"edges":        {Required: true},
	"walkers":      {},
	"start":        {Required: true},
	"done":         {Required: true},
	"finally":      {},
	"scorecard":    {},
	"calibration":  {},
}

// NodeFields registers every yaml field on NodeDef.
var NodeFields = FieldRegistry{
	"name":          {Required: true},
	"description":   {},
	"approach":      {ValidValues: ValidApproaches},
	"instrument":    {},
	"action":        {},
	"command":       {},
	"handler_type":  {ValidValues: ValidHandlerTypes}, // legacy — migration
	"handler":       {},                               // legacy — migration
	"timeout":       {},
	"provider":      {},
	"prompt":        {},
	"output_schema": {},
	"input":         {},
	"before":        {},
	"after":         {},
	"schema":        {},
	"cache":         {},
	"meta":          {},
	"code":          {},
	"display_name":  {},
	"output":        {},
}

// EdgeFields registers every yaml field on EdgeDef.
var EdgeFields = FieldRegistry{
	"id":           {},
	"name":         {},
	"from":         {Required: true},
	"to":           {Required: true},
	"shortcut":     {},
	"loop":         {},
	"parallel":     {},
	"condition":    {},
	"when":         {},
	"merge":        {ValidValues: ValidMergeStrategies},
	"display_name": {},
}

// ZoneFields registers every yaml field on ZoneDef.
var ZoneFields = FieldRegistry{
	"nodes":          {Required: true},
	"approach":       {ValidValues: ValidApproaches},
	"stickiness":     {},
	"domain":         {ValidValues: ValidZoneDomains},
	"context_filter": {},
}

// WalkerFields registers every yaml field on WalkerDef.
var WalkerFields = FieldRegistry{
	"name":            {Required: true},
	"approach":        {ValidValues: ValidApproaches},
	"persona":         {},
	"preamble":        {},
	"offset_preamble": {},
	"step_affinity":   {},
	"role":            {},
}

// InstrumentFields registers every yaml field on InstrumentManifest.
var InstrumentFields = FieldRegistry{
	"kind":        {Required: true},
	"name":        {Required: true},
	"namespace":   {Required: true},
	"version":     {},
	"description": {},
	"dispatch":    {Required: true, ValidValues: ValidDispatchModes},
	"tune":        {Required: true},
	"endpoint":    {},
	"image":       {},
	"actions":     {Required: true},
}

// PortFields registers every yaml field on PortDef.
var PortFields = FieldRegistry{
	"name":        {Required: true},
	"direction":   {Required: true, ValidValues: ValidPortDirections},
	"type":        {},
	"description": {},
}
