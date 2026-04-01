package main

import "errors"

var errOrchestrateDisabled = errors.New("orchestrate command disabled pending Jericho v0.2.0 migration (ORG-GOL-63)")

// orchestrateCmd is disabled pending Jericho v0.2.0 migration.
// Manager was deleted — replace with workload.Controller or direct RunWorker calls.
func orchestrateCmd(_ []string) error {
	return errOrchestrateDisabled
}
