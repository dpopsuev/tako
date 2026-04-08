package operator

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"

	oculusengine "github.com/dpopsuev/oculus/engine"
	oculusstore "github.com/dpopsuev/origami/instruments/oculus"
)

// GitObserver implements Observer by polling git HEAD and running an
// Oculus scan to snapshot the current repository state.
type GitObserver struct {
	repoPath string
	lastSHA  string
}

// NewGitObserver creates an observer for the given repository.
func NewGitObserver(repoPath string) *GitObserver {
	return &GitObserver{repoPath: repoPath}
}

// Observe snapshots the current state: HEAD SHA + Oculus scan findings.
func (g *GitObserver) Observe() (*CurrentState, error) {
	sha, err := gitHEAD(g.repoPath)
	if err != nil {
		return nil, fmt.Errorf("git HEAD: %w", err)
	}

	state := &CurrentState{
		HeadSHA:      sha,
		BuildPassing: true, // assume passing until circuit proves otherwise
		TestPassing:  true,
		ObservedAt:   time.Now(),
	}

	// Only scan if HEAD changed (avoid re-scanning the same commit).
	if sha == g.lastSHA {
		return state, nil
	}
	g.lastSHA = sha

	// Run Oculus scan directly — no dependency on sdlctype.
	store := &oculusstore.NopStore{}
	oc := oculusengine.New(store, []string{g.repoPath})
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	scanResult, err := oc.ScanProject(ctx, g.repoPath, oculusengine.ScanOpts{})
	if err != nil {
		return state, nil
	}
	state.ScanFindings = len(scanResult.Report.HotSpots) + len(scanResult.Report.Cycles)

	return state, nil
}

func gitHEAD(repoPath string) (string, error) {
	cmd := exec.Command("git", "rev-parse", "HEAD")
	cmd.Dir = repoPath
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

var _ Observer = (*GitObserver)(nil)
