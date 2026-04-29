package discourse

// Board is a named collection of Topics — the mailbox abstraction.
type Board struct {
	Name   string
	Topics []Topic
}

// Monolog is the agent's internal discourse (letters to self).
type Monolog interface {
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
