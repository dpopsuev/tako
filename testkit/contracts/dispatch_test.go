package contracts_test

import (
	"testing"

	"github.com/dpopsuev/tako/testkit/contracts"
)

func TestDispatcherContract(t *testing.T) {
	contracts.RunDispatcherContract(t)
}
