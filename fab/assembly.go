package fab

import "errors"

var (
	ErrStationNotFound = errors.New("fab: station not found")
	ErrNoIntake        = errors.New("fab: assembly has no intake station")
)

// Assembly is the Fab production line graph — stations connected by contracts.
type Assembly struct {
	Name      string
	Stations  map[string]Station
	Contracts []Contract
}

// Intake returns the intake station.
func (a Assembly) Intake() (Station, error) {
	for _, s := range a.Stations {
		if s.Intake {
			return s, nil
		}
	}
	return Station{}, ErrNoIntake
}

// Successors returns stations reachable from the named station via contracts.
func (a Assembly) Successors(name string) []string {
	var out []string
	for _, c := range a.Contracts {
		if c.From == name {
			out = append(out, c.To)
		}
	}
	return out
}

// ContractsFrom returns all contracts originating at the named station.
func (a Assembly) ContractsFrom(name string) []Contract {
	var out []Contract
	for _, c := range a.Contracts {
		if c.From == name {
			out = append(out, c)
		}
	}
	return out
}
