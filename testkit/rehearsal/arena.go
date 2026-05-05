package rehearsal

import (
	"context"
	"time"
)

type Scenario interface {
	ID() string
	Spec() string
	Timeout() time.Duration
	Budget() Budget
}

type Referee interface {
	Check(ctx context.Context, scenarioID, projectPath string) (CheckResult, error)
}

type Operator interface {
	Perform(ctx context.Context, message string) (string, error)
}

type CheckResult struct {
	Pass   bool     `json:"pass"`
	Score  float64  `json:"score"`
	Errors []string `json:"errors,omitempty"`
}

type Budget struct {
	MaxTokens int           `json:"max_tokens"`
	MaxCost   float64       `json:"max_cost"`
	MaxTime   time.Duration `json:"max_time"`
}

type RunMetrics struct {
	ScenarioID  string            `json:"scenario_id"`
	Scale       Scale             `json:"scale"`
	TimeElapsed time.Duration     `json:"time_elapsed"`
	TokensIn    int               `json:"tokens_in"`
	TokensOut   int               `json:"tokens_out"`
	Pass        bool              `json:"pass"`
	Score       float64           `json:"score"`
	Artifacts   map[string]string `json:"artifacts,omitempty"`
}
