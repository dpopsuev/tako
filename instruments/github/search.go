package github

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

const maxSearchResults = 50

// LocalSearchResult represents a single ripgrep match. This is the internal
// type used by the connector; git_driver.go maps these to skn.SearchResult.
type LocalSearchResult struct {
	File    string
	Line    int
	Snippet string
	Score   float64
}

// SearchCode runs ripgrep on the local clone and returns matching results.
func SearchCode(ctx context.Context, localPath string, keywords []string) ([]LocalSearchResult, error) {
	if len(keywords) == 0 {
		return nil, nil
	}

	pattern := strings.Join(keywords, "|")

	args := []string{
		"--json",
		"--max-count", "5",
		"--max-filesize", "1M",
		"--type-add", "code:*.{go,py,rs,js,ts,yaml,yml,json,sh,c,h,cpp,hpp}",
		"--type", "code",
		"-e", pattern,
		localPath,
	}

	cmd := exec.CommandContext(ctx, "rg", args...)
	output, err := cmd.Output()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) && exitErr.ExitCode() == 1 {
			return nil, nil
		}
		return nil, fmt.Errorf("ripgrep search: %w", err)
	}

	return parseRipgrepJSON(output, localPath)
}

type rgMessage struct {
	Type string          `json:"type"`
	Data json.RawMessage `json:"data"`
}

type rgMatch struct {
	Path    rgPath       `json:"path"`
	Lines   rgText       `json:"lines"`
	LineNum int          `json:"line_number"`
	SubM    []rgSubmatch `json:"submatches"`
}

type rgPath struct {
	Text string `json:"text"`
}

type rgText struct {
	Text string `json:"text"`
}

type rgSubmatch struct {
	Match rgText `json:"match"`
}

func parseRipgrepJSON(data []byte, basePath string) ([]LocalSearchResult, error) {
	lines := strings.Split(string(data), "\n")
	results := make([]LocalSearchResult, 0, len(lines))

	for _, line := range lines {
		if line == "" {
			continue
		}
		var msg rgMessage
		if err := json.Unmarshal([]byte(line), &msg); err != nil {
			continue
		}
		if msg.Type != "match" {
			continue
		}
		var m rgMatch
		if err := json.Unmarshal(msg.Data, &m); err != nil {
			continue
		}

		relPath := strings.TrimPrefix(m.Path.Text, basePath+"/")
		results = append(results, LocalSearchResult{
			File:    relPath,
			Line:    m.LineNum,
			Snippet: strings.TrimSpace(m.Lines.Text),
			Score:   float64(len(m.SubM)),
		})

		if len(results) >= maxSearchResults {
			break
		}
	}
	return results, nil
}
