package reactivity

// Node is a Reactor with Directives. Directive[0] is the default.
// Empty directives = gimped (pass through, no processing).
// A Node without directives is gimped (pass through).
type Node struct {
	phase      AtomType
	label      string
	directives []Directive
}

var _ Reactor = (*Node)(nil)

func NewNode(phase AtomType, label string, defaultDirective Directive) *Node {
	return &Node{
		phase:      phase,
		label:      label,
		directives: []Directive{defaultDirective},
	}
}

func GimpedNode(phase AtomType) *Node {
	return &Node{phase: phase, label: phase.String()}
}

func (n *Node) Label() string { return n.label }

func (n *Node) React(m *Molecule, _ Atom) (YieldKind, Yield) {
	return Pass, Yield{}
}

func (n *Node) Phase() AtomType         { return n.phase }
func (n *Node) Directives() []Directive { return n.directives }
func (n *Node) Gimped() bool            { return len(n.directives) == 0 }

func (n *Node) AddDirective(d Directive) {
	n.directives = append(n.directives, d)
}

func (n *Node) RemoveDirective(i int) {
	if i < 0 || i >= len(n.directives) {
		return
	}
	n.directives = append(n.directives[:i], n.directives[i+1:]...)
}
