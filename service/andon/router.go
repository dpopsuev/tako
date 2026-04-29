package andon

import (
	"context"
	"path"
	"sync"
	"time"
)

type RouteTarget string

const (
	TargetManager RouteTarget = "manager"
	TargetBroker  RouteTarget = "broker"
	TargetLog     RouteTarget = "log"
)

type RouteRule struct {
	Severity Severity    `json:"severity" yaml:"severity"`
	Domain   string      `json:"domain" yaml:"domain"`
	Target   RouteTarget `json:"target" yaml:"target"`
}

type FindingHandlers struct {
	Manager func(Finding)
	Broker  func(Finding)
	Log     func(Finding)
}

type FindingRouter struct {
	rules    []RouteRule
	handlers FindingHandlers

	mu       sync.RWMutex
	findings []Finding
}

func NewFindingRouter(rules []RouteRule, handlers FindingHandlers) *FindingRouter {
	return &FindingRouter{rules: rules, handlers: handlers}
}

func (r *FindingRouter) Report(_ context.Context, f *Finding) error {
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

func (r *FindingRouter) Findings() []Finding {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]Finding, len(r.findings))
	copy(out, r.findings)
	return out
}

func (r *FindingRouter) route(f *Finding) RouteTarget {
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

func (r *FindingRouter) defaultTarget(severity Severity) RouteTarget {
	switch severity {
	case SeverityError:
		return TargetBroker
	case SeverityWarning:
		return TargetManager
	default:
		return TargetLog
	}
}

func (r *FindingRouter) dispatch(target RouteTarget, f *Finding) {
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

var _ FindingCollector = (*FindingRouter)(nil)
var _ FindingCollector = (*InMemoryCollector)(nil)
