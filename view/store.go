package view

import (
	"strings"
	"sync"
	"time"

	"github.com/dpopsuev/origami/circuit"
	"github.com/dpopsuev/origami/agentport"
)

// CircuitStore is the single source of truth for a circuit's visual state.
// It implements circuit.WalkObserver, maintaining a CircuitSnapshot that
// reflects the current walk state. Subscribers receive StateDiff values
// as the state changes.
//
// Thread-safe: walk events arrive from walk goroutines while subscribers
// read diffs from their own goroutines.
type CircuitStore struct {
	mu          sync.RWMutex
	snapshot    CircuitSnapshot
	def         *circuit.CircuitDef
	subscribers map[int]chan StateDiff
	nextID      int
	closed      bool

	nodeZone    map[string]string
	nodeElement map[string]string
}

// NewCircuitStore creates a store initialized from a circuit definition.
// All nodes start in NodeIdle state.
func NewCircuitStore(def *circuit.CircuitDef) *CircuitStore {
	nodes := make(map[string]NodeState, len(def.Nodes))
	nodeZone := make(map[string]string)
	nodeElement := make(map[string]string)

	for zoneName, zd := range def.Zones {
		for _, nodeName := range zd.Nodes {
			nodeZone[string(nodeName)] = zoneName
		}
	}

	for i := range def.Nodes {
		nd := &def.Nodes[i]
		name := string(nd.Name)
		elem, _ := agentport.ResolveApproach(strings.ToLower(nd.Approach))
		elemStr := string(elem)
		nodes[name] = NodeState{
			Name:    name,
			State:   NodeIdle,
			Zone:    nodeZone[name],
			Element: elemStr,
		}
		nodeElement[name] = elemStr
	}

	return &CircuitStore{
		snapshot: CircuitSnapshot{
			CircuitName: def.Circuit,
			Nodes:       nodes,
			Walkers:     make(map[string]WalkerPosition),
			Cases:       make(map[string]CaseInfo),
			Breakpoints: make(map[string]bool),
			Timestamp:   time.Now().UTC(),
		},
		def:         def,
		subscribers: make(map[int]chan StateDiff),
		nodeZone:    nodeZone,
		nodeElement: nodeElement,
	}
}

// Reset clears all state and reinitializes the store from a new circuit
// definition. Existing subscribers are preserved and receive a DiffReset
// diff so they know to discard cached state. Used by Sumi's SSE client
// when reconnecting after a server-side session swap.
func (cs *CircuitStore) Reset(def *circuit.CircuitDef) {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	if def == nil {
		def = &circuit.CircuitDef{}
	}

	nodes := make(map[string]NodeState, len(def.Nodes))
	nodeZone := make(map[string]string)
	nodeElement := make(map[string]string)

	for zoneName, zd := range def.Zones {
		for _, nodeName := range zd.Nodes {
			nodeZone[string(nodeName)] = zoneName
		}
	}

	for i := range def.Nodes {
		nd := &def.Nodes[i]
		name := string(nd.Name)
		elem2, _ := agentport.ResolveApproach(strings.ToLower(nd.Approach))
		elemStr2 := string(elem2)
		nodes[name] = NodeState{
			Name:    name,
			State:   NodeIdle,
			Zone:    nodeZone[name],
			Element: elemStr2,
		}
		nodeElement[name] = elemStr2
	}

	now := time.Now().UTC()
	cs.snapshot = CircuitSnapshot{
		CircuitName: def.Circuit,
		Nodes:       nodes,
		Walkers:     make(map[string]WalkerPosition),
		Cases:       make(map[string]CaseInfo),
		Breakpoints: make(map[string]bool),
		Timestamp:   now,
	}
	cs.def = def
	cs.nodeZone = nodeZone
	cs.nodeElement = nodeElement

	cs.emit(&StateDiff{Type: DiffReset, Timestamp: now})
}

// Def returns the CircuitDef used to initialize this store.
func (cs *CircuitStore) Def() *circuit.CircuitDef {
	cs.mu.RLock()
	defer cs.mu.RUnlock()
	return cs.def
}

// OnEvent implements circuit.WalkObserver. It updates the snapshot and
// emits StateDiff values to subscribers.
func (cs *CircuitStore) OnEvent(we *circuit.WalkEvent) {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	now := time.Now().UTC()
	cs.snapshot.Timestamp = now

	switch we.Type {
	case circuit.EventNodeEnter:
		cs.setNodeState(we.Node, NodeActive, now)
		cs.moveWalker(we.Walker, we.Node, now)

	case circuit.EventNodeExit:
		cs.setNodeState(we.Node, NodeCompleted, now)

	case circuit.EventTransition:
		cs.moveWalker(we.Walker, we.Node, now)

	case circuit.EventWalkerSwitch:
		cs.moveWalker(we.Walker, we.Node, now)

	case circuit.EventFanOutStart:
		if we.Walker != "" {
			cs.addWalker(we.Walker, we.Node, now)
		}

	case circuit.EventFanOutEnd:
		if we.Walker != "" {
			cs.removeWalker(we.Walker, now)
		}

	case circuit.EventWalkComplete:
		if we.Walker != "" {
			if ci, ok := cs.snapshot.Cases[we.Walker]; ok {
				ci.State = CaseCompleted
				cs.snapshot.Cases[we.Walker] = ci
			}
		} else {
			for id, ci := range cs.snapshot.Cases {
				if ci.State == CaseActive {
					ci.State = CaseCompleted
					cs.snapshot.Cases[id] = ci
				}
			}
		}
		cs.clearWalkers(we.Walker, now)
		cs.snapshot.Completed = true
		cs.emit(&StateDiff{Type: DiffCompleted, Timestamp: now})

	case circuit.EventWalkError:
		errMsg := ""
		if we.Error != nil {
			errMsg = we.Error.Error()
		}
		if we.Node != "" {
			cs.setNodeState(we.Node, NodeError, now)
		}
		if we.Walker != "" {
			if ci, ok := cs.snapshot.Cases[we.Walker]; ok {
				ci.State = CaseError
				cs.snapshot.Cases[we.Walker] = ci
			}
		}
		cs.clearWalkers(we.Walker, now)
		cs.snapshot.Error = errMsg
		cs.emit(&StateDiff{Type: DiffError, Node: we.Node, Error: errMsg, Timestamp: now})

	case circuit.EventWalkInterrupted:
		cs.snapshot.Paused = true
		cs.emit(&StateDiff{Type: DiffPaused, Node: we.Node, Timestamp: now})

	case circuit.EventWalkResumed:
		cs.snapshot.Paused = false
		cs.emit(&StateDiff{Type: DiffResumed, Timestamp: now})
	}
}

// Snapshot returns a thread-safe copy of the current circuit state.
func (cs *CircuitStore) Snapshot() CircuitSnapshot {
	cs.mu.RLock()
	defer cs.mu.RUnlock()

	nodes := make(map[string]NodeState, len(cs.snapshot.Nodes))
	for k, v := range cs.snapshot.Nodes {
		nodes[k] = v
	}

	walkers := make(map[string]WalkerPosition, len(cs.snapshot.Walkers))
	for k, v := range cs.snapshot.Walkers {
		walkers[k] = v
	}

	breakpoints := make(map[string]bool, len(cs.snapshot.Breakpoints))
	for k, v := range cs.snapshot.Breakpoints {
		breakpoints[k] = v
	}

	cases := make(map[string]CaseInfo, len(cs.snapshot.Cases))
	for k, v := range cs.snapshot.Cases {
		cases[k] = v
	}

	return CircuitSnapshot{
		CircuitName: cs.snapshot.CircuitName,
		Def:         cs.def,
		Nodes:       nodes,
		Walkers:     walkers,
		Cases:       cases,
		Breakpoints: breakpoints,
		Paused:      cs.snapshot.Paused,
		Completed:   cs.snapshot.Completed,
		Error:       cs.snapshot.Error,
		Timestamp:   cs.snapshot.Timestamp,
	}
}

// Subscribe returns a channel that receives all future StateDiff values.
// Call Unsubscribe with the returned id when done. If the store is
// already closed, the returned channel is immediately closed.
func (cs *CircuitStore) Subscribe() (id int, ch <-chan StateDiff) {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	c := make(chan StateDiff, 512)
	id = cs.nextID
	cs.nextID++
	if cs.closed {
		close(c)
		return id, c
	}
	cs.subscribers[id] = c
	return id, c
}

// Unsubscribe removes a subscriber and closes its channel.
func (cs *CircuitStore) Unsubscribe(id int) {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	if c, ok := cs.subscribers[id]; ok {
		close(c)
		delete(cs.subscribers, id)
	}
}

// SetBreakpoints replaces the breakpoint set. Called by debug controllers
// (e.g. kami.Server) without requiring a kami import.
func (cs *CircuitStore) SetBreakpoints(nodes []string) {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	now := time.Now().UTC()

	old := cs.snapshot.Breakpoints
	cs.snapshot.Breakpoints = make(map[string]bool, len(nodes))
	for _, n := range nodes {
		cs.snapshot.Breakpoints[n] = true
		if !old[n] {
			cs.emit(&StateDiff{Type: DiffBreakpointSet, Node: n, Timestamp: now})
		}
	}
	for n := range old {
		if !cs.snapshot.Breakpoints[n] {
			cs.emit(&StateDiff{Type: DiffBreakpointCleared, Node: n, Timestamp: now})
		}
	}
}

// SetPaused sets the paused state. Called by debug controllers.
func (cs *CircuitStore) SetPaused(paused bool) {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	if cs.snapshot.Paused == paused {
		return
	}
	now := time.Now().UTC()
	cs.snapshot.Paused = paused
	if paused {
		cs.emit(&StateDiff{Type: DiffPaused, Timestamp: now})
	} else {
		cs.emit(&StateDiff{Type: DiffResumed, Timestamp: now})
	}
}

// Close closes all subscriber channels.
func (cs *CircuitStore) Close() {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	if cs.closed {
		return
	}
	cs.closed = true
	for id, ch := range cs.subscribers {
		close(ch)
		delete(cs.subscribers, id)
	}
}

func (cs *CircuitStore) setNodeState(node string, state NodeVisualState, ts time.Time) {
	if ns, ok := cs.snapshot.Nodes[node]; ok {
		ns.State = state
		cs.snapshot.Nodes[node] = ns
	}
	cs.emit(&StateDiff{Type: DiffNodeState, Node: node, State: state, Timestamp: ts})
}

func (cs *CircuitStore) moveWalker(walkerID, node string, ts time.Time) {
	if walkerID == "" {
		return
	}
	wp, exists := cs.snapshot.Walkers[walkerID]
	if !exists {
		cs.addWalker(walkerID, node, ts)
		return
	}
	if wp.Node == node {
		return
	}
	wp.Node = node
	cs.snapshot.Walkers[walkerID] = wp
	if ci, ok := cs.snapshot.Cases[walkerID]; ok {
		ci.Node = node
		ci.Element = cs.nodeElement[node]
		cs.snapshot.Cases[walkerID] = ci
	}
	cs.emit(&StateDiff{Type: DiffWalkerMoved, Walker: walkerID, Node: node, Timestamp: ts})
}

func (cs *CircuitStore) addWalker(walkerID, node string, ts time.Time) {
	cs.snapshot.Walkers[walkerID] = WalkerPosition{
		WalkerID: walkerID,
		Node:     node,
		Element:  cs.nodeElement[node],
	}
	if _, exists := cs.snapshot.Cases[walkerID]; !exists {
		cs.snapshot.Cases[walkerID] = CaseInfo{
			CaseID:  walkerID,
			State:   CaseActive,
			Node:    node,
			Element: cs.nodeElement[node],
		}
	}
	cs.emit(&StateDiff{Type: DiffWalkerAdded, Walker: walkerID, Node: node, Timestamp: ts})
}

func (cs *CircuitStore) removeWalker(walkerID string, ts time.Time) {
	delete(cs.snapshot.Walkers, walkerID)
	cs.emit(&StateDiff{Type: DiffWalkerRemoved, Walker: walkerID, Timestamp: ts})
}

// clearWalkers removes walkers on terminal events (complete, error).
// If walkerID is non-empty, only that walker is removed.
// If walkerID is empty, all walkers are removed (session-level event).
func (cs *CircuitStore) clearWalkers(walkerID string, ts time.Time) {
	if walkerID != "" {
		if _, exists := cs.snapshot.Walkers[walkerID]; exists {
			cs.removeWalker(walkerID, ts)
		}
		return
	}
	for wid := range cs.snapshot.Walkers {
		cs.removeWalker(wid, ts)
	}
}

// emit broadcasts a diff to all subscribers. Non-blocking: slow subscribers
// that fall behind have diffs dropped. Must be called with cs.mu held.
func (cs *CircuitStore) emit(diff *StateDiff) {
	for _, ch := range cs.subscribers {
		select {
		case ch <- *diff:
		default:
		}
	}
}
