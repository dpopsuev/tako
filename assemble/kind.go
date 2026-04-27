package assemble

// Kind is implemented by the three top-level DSL types.
type Kind interface {
	KindName() string
}

// Complex is a thin composition root — references + wiring of other Fabs.
type Complex struct {
	Name string
	Refs []FabRef
}

func (c Complex) KindName() string { return "Complex" }

// FabRef is a reference to a Fab within a Complex.
type FabRef struct {
	Name   string
	Module string
}

// Fab is the substance — stations, contracts, instruments inline.
type Fab struct {
	Name      string
	Stations  []StationDef
	Contracts []ContractDef
}

func (f Fab) KindName() string { return "Fab" }

// StationDef is a station definition in a Fab YAML.
type StationDef struct {
	Name        string   `yaml:"name"`
	Instruments []string `yaml:"instruments"`
	Intake      bool     `yaml:"intake,omitempty"`
	Terminus    bool     `yaml:"terminus,omitempty"`
}

// ContractDef is a contract definition in a Fab YAML.
type ContractDef struct {
	From   string `yaml:"from"`
	To     string `yaml:"to"`
	Assert string `yaml:"assert,omitempty"`
}

// Rehearsal is a validation kind — knowledge, judgment, station, fab, complex levels.
type Rehearsal struct {
	Name  string
	Level string
}

func (r Rehearsal) KindName() string { return "Rehearsal" }
