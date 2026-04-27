package andon

import "sync"

// StubSignal is an always-green Andon — Pull is tracked but never escalates.
type StubSignal struct {
	mu    sync.Mutex
	color Color
	pulls int
}

var _ Signal = (*StubSignal)(nil)

func (s *StubSignal) Pull(_ string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.pulls++
}

func (s *StubSignal) Status() Color {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.color
}

func (s *StubSignal) Reset() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.color = Green
	s.pulls = 0
}

// Pulls returns how many times the cord was pulled.
func (s *StubSignal) Pulls() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.pulls
}
