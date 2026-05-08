package cerebrum

import "github.com/dpopsuev/tako/agent/reactivity"

type SealCheck struct {
	Turn          int
	HasDesired    bool
	HasToolCalls  bool
	Content       string
	Distance      float64
	DeltaDistance  float64
	Molecule      *reactivity.Molecule
}

type SealStrategy interface {
	ShouldSeal(check SealCheck) bool
}

type ImmediateSeal struct{}

func (ImmediateSeal) ShouldSeal(c SealCheck) bool {
	return !c.HasDesired && !c.HasToolCalls
}

type ConsecutiveSeal struct {
	streak int
}

func (s *ConsecutiveSeal) ShouldSeal(c SealCheck) bool {
	if c.HasToolCalls {
		s.streak = 0
		return false
	}
	if c.HasDesired {
		return false
	}
	s.streak++
	return s.streak >= 2
}

type StagnantSeal struct {
	prevDistance float64
	streak      int
}

func (s *StagnantSeal) ShouldSeal(c SealCheck) bool {
	if c.HasToolCalls {
		s.streak = 0
		s.prevDistance = c.Distance
		return false
	}
	if c.HasDesired && c.DeltaDistance < 0 {
		s.streak = 0
		s.prevDistance = c.Distance
		return false
	}
	if !c.HasDesired {
		return true
	}
	if c.Distance == s.prevDistance {
		s.streak++
	} else {
		s.streak = 0
	}
	s.prevDistance = c.Distance
	return s.streak >= 2
}
