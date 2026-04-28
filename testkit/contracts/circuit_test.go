package contracts_test

import (
	"testing"

	"github.com/dpopsuev/tako/testkit/contracts"
)

func TestCircuitDefContract(t *testing.T) {
	contracts.RunCircuitDefContract(t, func() []byte {
		return []byte(`
circuit: test
start: scan
done: _done
nodes:
  - name: scan
    instrument: transformer
    action: echo
  - name: fix
    instrument: transformer
    action: echo
edges:
  - id: e1
    from: scan
    to: fix
  - id: e2
    from: fix
    to: _done
`)
	})
}

func TestArtifactValidationContract(t *testing.T) {
	contracts.RunArtifactValidationContract(t)
}
