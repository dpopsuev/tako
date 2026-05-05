package cerebrum

import "github.com/dpopsuev/tako/agent/reactivity"

type Recollector interface {
	Recollect(need []byte) []reactivity.Atom
}
