package rehearsal

import "context"

type GameReferee struct {
	IsSolved func(state map[string]any) bool
	State    func() map[string]any
}

var _ Referee = (*GameReferee)(nil)

func (r *GameReferee) Check(_ context.Context, _, _ string) (CheckResult, error) {
	if r.State == nil || r.IsSolved == nil {
		return CheckResult{Pass: false, Score: 0, Errors: []string{"GameReferee not configured"}}, nil
	}
	state := r.State()
	if r.IsSolved(state) {
		return CheckResult{Pass: true, Score: 1.0}, nil
	}
	return CheckResult{Pass: false, Score: 0.5, Errors: []string{"game not solved"}}, nil
}
