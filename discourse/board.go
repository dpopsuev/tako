package discourse

// Board is a named collection of Topics — the mailbox abstraction.
type Board struct {
	Name   string
	Topics []Topic
}

// Monologue is the agent's internal discourse (letters to self).
type Monologue interface {
	Pin(topic string)
	Focus(topic string)
	Write(letter Letter)
	Letters() []Letter
}

// Dialog is external discourse (letters to others).
type Dialog interface {
	Send(letter Letter) error
	Receive() (Letter, bool)
}
