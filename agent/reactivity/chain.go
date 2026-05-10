package reactivity

import (
	"fmt"
	"time"
)

type EventRole int

const (
	Sense EventRole = iota
	Motor
)

func (r EventRole) String() string {
	return [...]string{"sense", "motor"}[r]
}

type ChainEvent struct {
	ID         string
	ParentID   string
	Kind       EventRole
	Organ      string
	Input      []byte
	Output     []byte
	Phase      AtomType
	Timestamp  time.Time
	IsResponse bool
}

type EventChain struct {
	events []ChainEvent
	seq    int
}

func NewEventChain() *EventChain {
	return &EventChain{}
}

func (c *EventChain) Append(e ChainEvent) {
	if e.ID == "" {
		c.seq++
		e.ID = fmt.Sprintf("ev-%d", c.seq)
	}
	if e.ParentID == "" && len(c.events) > 0 {
		e.ParentID = c.events[len(c.events)-1].ID
	}
	if e.Timestamp.IsZero() {
		e.Timestamp = time.Now()
	}
	c.events = append(c.events, e)
}

func (c *EventChain) Recent(n int) []ChainEvent {
	if n <= 0 || n >= len(c.events) {
		return append([]ChainEvent(nil), c.events...)
	}
	return append([]ChainEvent(nil), c.events[len(c.events)-n:]...)
}

func (c *EventChain) All() []ChainEvent {
	return append([]ChainEvent(nil), c.events...)
}

func (c *EventChain) Motors() []ChainEvent {
	return c.byKind(Motor)
}

func (c *EventChain) Senses() []ChainEvent {
	return c.byKind(Sense)
}

func (c *EventChain) HasSenseAfterMotor() bool {
	lastMotor := -1
	for i := len(c.events) - 1; i >= 0; i-- {
		if c.events[i].Kind == Motor {
			lastMotor = i
			break
		}
	}
	if lastMotor < 0 {
		return false
	}
	for i := lastMotor + 1; i < len(c.events); i++ {
		if c.events[i].Kind == Sense {
			return true
		}
	}
	return false
}

func (c *EventChain) HasResponse() bool {
	for _, e := range c.events {
		if e.IsResponse {
			return true
		}
	}
	return false
}

func (c *EventChain) Len() int {
	return len(c.events)
}

func (c *EventChain) Last() (ChainEvent, bool) {
	if len(c.events) == 0 {
		return ChainEvent{}, false
	}
	return c.events[len(c.events)-1], true
}

func (c *EventChain) byKind(kind EventRole) []ChainEvent {
	var out []ChainEvent
	for _, e := range c.events {
		if e.Kind == kind {
			out = append(out, e)
		}
	}
	return out
}
