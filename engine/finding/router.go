package finding

// Category: Processing & Support

import (
	"context"
	"path"
	"sync"
	"time"

	"github.com/dpopsuev/tako/circuit"
)

// RouteTarget identifies the authority that receives a routed finding.
type RouteTarget string

const (
	TargetManager RouteTarget = "manager"
	TargetBroker  RouteTarget = "broker"
	TargetLog     RouteTarget = "log"
)

// RouteRule maps a severity + domain pattern to a target authority.
// Domain uses path.Match glob syntax (e.g. "test.*", "security.*").
// Rules are evaluated in order; first match wins.
type RouteRule struct {
	Severity circuit.FindingSeverity `json:"severity" yaml:"severity"`
	Domain   string                  `json:"domain" yaml:"domain"`
	Target   RouteTarget             `json:"target" yaml:"target"`
}

// FindingHandlers holds callbacks invoked when a finding is routed to a target.
type FindingHandlers struct {
	Manager func(circuit.Finding)
	Broker  func(circuit.Finding)
	Log     func(circuit.Finding)
}

// FindingRouter routes findings to the appropriate authority based on severity
// and domain, then collects them. It implements FindingCollector.
type FindingRouter struct {
	rules    []RouteRule
	handlers FindingHandlers

	mu       sync.RWMutex
	findings []circuit.Finding
}

// NewFindingRouter creates a router with the given rules and handlers.
// Default routing (when no rule matches): info->log, warning->manager, error->broker.
func NewFindingRouter(rules []RouteRule, handlers FindingHandlers) *FindingRouter {
	return &FindingRouter{rules: rules, handlers: handlers}
}

func (r *FindingRouter) Report(_ context.Context, f *circuit.Finding) error {
	if f.Timestamp.IsZero() {
		f.Timestamp = time.Now().UTC()
	}

	target := r.route(f)
	r.dispatch(target, f)

	r.mu.Lock()
	defer r.mu.Unlock()
	r.findings = append(r.findings, *f)
	return nil
}

func (r *FindingRouter) Findings() []circuit.Finding {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]circuit.Finding, len(r.findings))
	copy(out, r.findings)
	return out
}

func (r *FindingRouter) route(f *circuit.Finding) RouteTarget {
	for _, rule := range r.rules {
		if rule.Severity != "" && rule.Severity != f.Severity {
			continue
		}
		if rule.Domain != "" {
			matched, _ := path.Match(rule.Domain, f.Domain)
			if !matched {
				continue
			}
		}
		return rule.Target
	}
	return r.defaultTarget(f.Severity)
}

func (r *FindingRouter) defaultTarget(severity circuit.FindingSeverity) RouteTarget {
	switch severity {
	case circuit.FindingError:
		return TargetBroker
	case circuit.FindingWarning:
		return TargetManager
	default:
		return TargetLog
	}
}

func (r *FindingRouter) dispatch(target RouteTarget, f *circuit.Finding) {
	switch target {
	case TargetManager:
		if r.handlers.Manager != nil {
			r.handlers.Manager(*f)
		}
	case TargetBroker:
		if r.handlers.Broker != nil {
			r.handlers.Broker(*f)
		}
	case TargetLog:
		if r.handlers.Log != nil {
			r.handlers.Log(*f)
		}
	}
}
