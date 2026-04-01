package agentport

import "github.com/dpopsuev/jericho/workload"

// Workload type aliases — declarative agent group definitions.
type (
	WorkerPool   = workload.WorkerPool
	DebateTeam   = workload.DebateTeam
	TaskRunner   = workload.TaskRunner
	WorkloadKind = workload.Kind
	Controller   = workload.Controller
)

// Workload kind constants.
const (
	KindWorkerPool = workload.KindWorkerPool
	KindDebateTeam = workload.KindDebateTeam
	KindTaskRunner = workload.KindTaskRunner
)

// Constructors.
var (
	NewController = workload.NewController
	ParseWorkload = workload.Parse
)
