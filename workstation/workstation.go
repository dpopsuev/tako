package workstation

import (
	"github.com/dpopsuev/tako/fab"
	"github.com/dpopsuev/tako/instrument"
)

// Workstation is the execution environment for one agent at one station.
// The station declares instruments; the workstation provisions the shell.
type Workstation interface {
	Provision(station fab.Station) error
	Shell() instrument.Shell
	Reset() error
}
