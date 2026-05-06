package testkit

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

type CheckResult struct {
	Compiles    bool
	TestsPass   bool
	FileExists  map[string]bool
	FileContains map[string]bool
	GitCommits  int
	Errors      []string
}

func (r CheckResult) Pass() bool {
	return len(r.Errors) == 0
}

type GoReferee struct {
	WorkDir string
}

func (r GoReferee) Check(ctx context.Context) CheckResult {
	result := CheckResult{
		FileExists:   make(map[string]bool),
		FileContains: make(map[string]bool),
	}

	buildOut, buildErr := r.run(ctx, "go", "build", "./...")
	result.Compiles = buildErr == nil
	if buildErr != nil {
		result.Errors = append(result.Errors, "build failed: "+string(buildOut))
	}

	testOut, testErr := r.run(ctx, "go", "test", "./...", "-count=1", "-timeout=30s")
	result.TestsPass = testErr == nil
	if testErr != nil {
		result.Errors = append(result.Errors, "tests failed: "+string(testOut))
	}

	gitOut, _ := r.run(ctx, "git", "rev-list", "--count", "HEAD")
	if commits := strings.TrimSpace(string(gitOut)); commits != "" {
		var n int
		for _, c := range commits {
			if c >= '0' && c <= '9' {
				n = n*10 + int(c-'0')
			}
		}
		result.GitCommits = n
	}

	return result
}

func (r GoReferee) AssertFileExists(result *CheckResult, relPath string) {
	_, err := os.Stat(filepath.Join(r.WorkDir, relPath))
	exists := err == nil
	result.FileExists[relPath] = exists
	if !exists {
		result.Errors = append(result.Errors, "file not found: "+relPath)
	}
}

func (r GoReferee) AssertFileContains(result *CheckResult, relPath, substr string) {
	data, err := os.ReadFile(filepath.Join(r.WorkDir, relPath))
	if err != nil {
		result.FileContains[relPath+"::"+substr] = false
		result.Errors = append(result.Errors, "cannot read "+relPath+": "+err.Error())
		return
	}
	contains := strings.Contains(string(data), substr)
	result.FileContains[relPath+"::"+substr] = contains
	if !contains {
		result.Errors = append(result.Errors, relPath+" missing: "+substr)
	}
}

func (r GoReferee) AssertFileNotContains(result *CheckResult, relPath, substr string) {
	data, err := os.ReadFile(filepath.Join(r.WorkDir, relPath))
	if err != nil {
		return
	}
	if strings.Contains(string(data), substr) {
		result.Errors = append(result.Errors, relPath+" should not contain: "+substr)
	}
}

func (r GoReferee) AssertNewCommit(result *CheckResult, baseline int) {
	if result.GitCommits <= baseline {
		result.Errors = append(result.Errors, "no new git commits")
	}
}

func (r GoReferee) run(ctx context.Context, name string, args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = r.WorkDir
	return cmd.CombinedOutput()
}
