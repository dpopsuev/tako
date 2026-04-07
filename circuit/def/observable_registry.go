package def

// EventDef describes an observable event in the circuit lifecycle.
// Observable events are decision points where system behavior changes —
// node transitions, edge evaluations, dispatch round-trips, session lifecycle.
// Informational log messages (warnings, config notes) are NOT events.
type EventDef struct {
	// Category groups related events for filtering.
	Category string
	// Fields lists the structured slog fields this event must carry.
	Fields []string
}

// EventRegistry maps event constant values to their definitions.
// The trap test ensures every observable event constant has an entry.
type EventRegistry map[string]EventDef

// Has returns true if the event is registered.
func (r EventRegistry) Has(event string) bool {
	_, ok := r[event]
	return ok
}

// ObservableEvents registers every observable event in the circuit lifecycle.
// Add an event constant without registering it here → trap test fails.
var ObservableEvents = EventRegistry{
	// Walk events — circuit graph traversal
	"node enter":        {Category: "walk", Fields: []string{"node", "walker"}},
	"node exit":         {Category: "walk", Fields: []string{"node", "walker", "elapsed_ms"}},
	"edge taken":        {Category: "walk", Fields: []string{"edge", "from", "to"}},
	"no matching edge":  {Category: "walk", Fields: []string{"node"}},
	"loop incremented":  {Category: "walk", Fields: []string{"node", "loop"}},
	"walk complete":     {Category: "walk", Fields: []string{"walker"}},
	"walk error":        {Category: "walk", Fields: []string{"walker", "error"}},
	"delegate start":    {Category: "walk", Fields: []string{"node"}},
	"delegate complete": {Category: "walk", Fields: []string{"node"}},

	// DSL events — circuit loading and composition
	"sub-circuit loaded":     {Category: "dsl", Fields: []string{"circuit", "nodes"}},
	"overlay merge":          {Category: "dsl", Fields: []string{"circuit"}},
	"overlay merge complete": {Category: "dsl", Fields: []string{"circuit", "merged_nodes", "merged_edges"}},
	"merge components":       {Category: "build", Fields: []string{"components"}},

	// Calibrate events — calibration run lifecycle
	"calibration run start": {Category: "calibrate", Fields: []string{"run", "total"}},
	"case complete":         {Category: "calibrate", Fields: []string{"case_id"}},
	"starting run":          {Category: "calibrate", Fields: []string{"run", "total"}},
	"case walk failed":      {Category: "calibrate", Fields: []string{"case_id", "error"}},

	// Dispatch events — work distribution
	"dispatch begin":          {Category: "dispatch", Fields: []string{"dispatch_id", "case_id", "step"}},
	"dispatch round-trip":     {Category: "dispatch", Fields: []string{"dispatch_id", "elapsed_ms"}},
	"dispatch timeout":        {Category: "dispatch", Fields: []string{"dispatch_id", "timeout"}},
	"step complete":           {Category: "dispatch", Fields: []string{"dispatch_id", "case_id", "step"}},
	"mux dispatch registered": {Category: "dispatch", Fields: []string{"dispatch_id"}},
	"mux artifact routed":     {Category: "dispatch", Fields: []string{"dispatch_id"}},
	"mux dispatcher abort":    {Category: "dispatch", Fields: []string{}},

	// Session events — MCP circuit server lifecycle
	"circuit session started":   {Category: "session", Fields: []string{"session_id", "total_cases", "scenario"}},
	"circuit session failed":    {Category: "session", Fields: []string{"error"}},
	"circuit complete":          {Category: "session", Fields: []string{"session_id"}},
	"circuit run complete":      {Category: "session", Fields: []string{}},
	"step dispatched to worker": {Category: "session", Fields: []string{"dispatch_id", "step", "case_id"}},
	"step artifact accepted":    {Category: "session", Fields: []string{"dispatch_id", "step"}},
	"step delivered":            {Category: "session", Fields: []string{"dispatch_id", "step"}},

	// Worker events
	"workers spawned":   {Category: "worker", Fields: []string{"count"}},
	"worker registered": {Category: "worker", Fields: []string{"worker_id"}},

	// Transformer events
	"transformer executing": {Category: "transformer", Fields: []string{"transformer", "node"}},
	"transformer completed": {Category: "transformer", Fields: []string{"transformer", "node", "elapsed_ms"}},
	"transformer failed":    {Category: "transformer", Fields: []string{"transformer", "node", "error"}},

	// Health events
	"component health check": {Category: "health", Fields: []string{"name"}},
	"component healthy":      {Category: "health", Fields: []string{"name"}},
	"component unhealthy":    {Category: "health", Fields: []string{"name", "error"}},
	"all components healthy": {Category: "health", Fields: []string{"count"}},

	// Fold events
	"export validation complete": {Category: "fold", Fields: []string{"name", "count"}},

	// Observability events
	"observability metrics computed": {Category: "session", Fields: []string{"latency_nodes", "error_rate_nodes"}},

	// Signal events
	"signal emitted": {Category: "signal", Fields: []string{"event", "agent"}},

	// HITL events
	"inspect checkpoint": {Category: "hitl", Fields: []string{"walker_id"}},
	"resume walk":        {Category: "hitl", Fields: []string{"walker_id"}},
}
