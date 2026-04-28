package sdlc

// OrigamiLayers is the explicit layer ordering for the Tako codebase.
// Set via Locus desired state. Bottom (foundation) to top (consumers).
// Violations are imports from a lower layer to a higher layer.
var OrigamiLayers = []string{
	// Foundation
	"circuit/def",
	"circuit",
	// Shared
	"roster",
	"budget",
	"resource",
	"prompt",
	"report",
	// Engine
	"engine/gate",
	"engine/handler",
	"engine/trace",
	"engine/telemetry",
	"engine",
	// Dispatch
	"dispatch",
	"dispatch/guard",
	// Analysis
	"lint",
	"calibrate",
	"fold",
	"instruments/core",
	// Adapters
	"lsp",
	"autodoc",
	"domainserve",
	// Infrastructure
	"mcp",
	// Instruments
	"instruments/oculus",
	"instruments/gotools",
	"instruments/llmfix",
	"instruments/selfreview",
	// Simulation
	"simulate/sdlc/sdlctype",
	"simulate/sdlc",
	// Operator
	"operator",
	// Test
	"testkit",
	"testkit/stubs",
	"testkit/builders",
	"testkit/contracts",
	"testkit/assertions",
	"testkit/acceptance",
	// Binaries
	"cmd/origami",
	"cmd/operator",
	"cmd/agent-worker",
}
