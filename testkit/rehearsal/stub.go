package rehearsal

import (
	"context"
	"sync"
	"time"
)

type StubReferee struct {
	mu     sync.Mutex
	result CheckResult
	Checks int
}

func NewStubReferee(result CheckResult) *StubReferee {
	return &StubReferee{result: result}
}

func (r *StubReferee) Check(_ context.Context, _, _ string) (CheckResult, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.Checks++
	return r.result, nil
}

type StubScenario struct {
	id      string
	spec    string
	timeout time.Duration
	budget  Budget
}

func NewStubScenario(id, spec string) *StubScenario {
	return &StubScenario{
		id:      id,
		spec:    spec,
		timeout: 120 * time.Second,
	}
}

func (s *StubScenario) ID() string            { return s.id }
func (s *StubScenario) Spec() string          { return s.spec }
func (s *StubScenario) Timeout() time.Duration { return s.timeout }
func (s *StubScenario) Budget() Budget         { return s.budget }

type MockOperator struct {
	mu      sync.Mutex
	prompts []string
	current int
	Calls   int
}

func NewMockOperator(prompts ...string) *MockOperator {
	return &MockOperator{prompts: prompts}
}

func (o *MockOperator) Perform(_ context.Context, _ string) (string, error) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.Calls++
	if o.current >= len(o.prompts) {
		return "", nil
	}
	p := o.prompts[o.current]
	o.current++
	return p, nil
}
