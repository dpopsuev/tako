package agentport

import "github.com/dpopsuev/bugle/acp"

// ACP type aliases — definitions live in bugle/acp.
type (
	ACPLauncher    = acp.ACPLauncher
	ACPClient      = acp.Client
	ACPClientInfo  = acp.ClientInfo
	CommandFactory = acp.CommandFactory
)

// ACP constructors.
var (
	NewACPLauncher = acp.NewACPLauncher
	ACPAgentCommands = acp.AgentCommands
)
