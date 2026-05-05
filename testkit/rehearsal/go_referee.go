package rehearsal

import (
	"context"
	"fmt"
	"os/exec"
)

type GoTestReferee struct{}

var _ Referee = (*GoTestReferee)(nil)

func (r *GoTestReferee) Check(ctx context.Context, _, projectPath string) (CheckResult, error) {
	var errs []string

	buildCmd := exec.CommandContext(ctx, "go", "build", "./...")
	buildCmd.Dir = projectPath
	buildOut, err := buildCmd.CombinedOutput()
	if err != nil {
		errs = append(errs, fmt.Sprintf("build: %s\n%s", err, string(buildOut)))
		return CheckResult{Pass: false, Score: 0, Errors: errs}, nil
	}

	testCmd := exec.CommandContext(ctx, "go", "test", "./...", "-count=1")
	testCmd.Dir = projectPath
	testOut, err := testCmd.CombinedOutput()
	if err != nil {
		errs = append(errs, fmt.Sprintf("test: %s\n%s", err, string(testOut)))
		return CheckResult{Pass: false, Score: 0.5, Errors: errs}, nil
	}

	return CheckResult{Pass: true, Score: 1.0}, nil
}
