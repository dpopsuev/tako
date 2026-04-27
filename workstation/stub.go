package workstation

import (
	"github.com/dpopsuev/origami/fab"
	"github.com/dpopsuev/origami/instrument"
)

// StubWorkstation provisions a StubShell from the station's instrument manifest.
type StubWorkstation struct {
	shell   instrument.Shell
	station fab.Station
}

var _ Workstation = (*StubWorkstation)(nil)

func NewStubWorkstation() *StubWorkstation {
	return &StubWorkstation{}
}

func (w *StubWorkstation) Provision(station fab.Station) error {
	w.station = station
	w.shell = instrument.NewStubShell()
	return nil
}

func (w *StubWorkstation) Shell() instrument.Shell {
	return w.shell
}

func (w *StubWorkstation) Reset() error {
	w.shell = nil
	w.station = fab.Station{}
	return nil
}
