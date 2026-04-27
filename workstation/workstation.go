package workstation

import (
	"github.com/dpopsuev/origami/fab"
	"github.com/dpopsuev/origami/instrument"
)

// Workstation is the execution environment for one agent at one station.
// The station declares instruments; the workstation provisions the shell.
type Workstation interface {
	Provision(station fab.Station) error
	Shell() instrument.Shell
	Reset() error
}
