package discourse

import "sync"

// StubMonolog is an in-memory monolog — appends letters, no pressure model.
type StubMonolog struct {
	mu      sync.Mutex
	pinned  string
	focused string
	letters []Letter
}

var _ Monolog = (*StubMonolog)(nil)

func (m *StubMonolog) Pin(topic string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.pinned = topic
}

func (m *StubMonolog) Focus(topic string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.focused = topic
}

func (m *StubMonolog) Write(letter Letter) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.letters = append(m.letters, letter)
}

func (m *StubMonolog) Letters() []Letter {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]Letter(nil), m.letters...)
}

// StubDialog is an in-memory dialog — buffered channel semantics.
type StubDialog struct {
	mu     sync.Mutex
	inbox  []Letter
	outbox []Letter
}

var _ Dialog = (*StubDialog)(nil)

func (d *StubDialog) Send(letter Letter) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.outbox = append(d.outbox, letter)
	return nil
}

func (d *StubDialog) Receive() (Letter, bool) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if len(d.inbox) == 0 {
		return Letter{}, false
	}
	letter := d.inbox[0]
	d.inbox = d.inbox[1:]
	return letter, true
}
