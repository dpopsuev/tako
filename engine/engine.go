package engine

// Dispatcher defines the interface for dispatching work,
// allowing engine to remain independent of the dispatch package.
type Dispatcher interface {
	Dispatch(task Task) error
}

// Task represents a unit of work to be dispatched.
type Task struct {
	ID      string
	Payload interface{}
}

// Engine processes tasks using a Dispatcher.
type Engine struct {
	dispatcher Dispatcher
}

// New creates a new Engine with the given Dispatcher.
func New(d Dispatcher) *Engine {
	return &Engine{dispatcher: d}
}

// Run executes the engine logic, dispatching tasks via the Dispatcher interface.
func (e *Engine) Run(task Task) error {
	return e.dispatcher.Dispatch(task)
}
