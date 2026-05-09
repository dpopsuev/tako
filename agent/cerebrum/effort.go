package cerebrum

import (
	"strings"
	"unicode"
)

func isConversational(need []byte) bool {
	s := strings.TrimSpace(string(need))
	words := countWords(s)

	if words > 5 {
		return false
	}

	if strings.Contains(s, "?") {
		return false
	}

	lower := strings.ToLower(s)
	for _, kw := range actionKeywords {
		if strings.Contains(lower, kw) {
			return false
		}
	}

	return true
}

func countWords(s string) int {
	n := 0
	inWord := false
	for _, r := range s {
		if unicode.IsSpace(r) {
			inWord = false
		} else if !inWord {
			inWord = true
			n++
		}
	}
	return n
}

var actionKeywords = []string{
	"read", "write", "edit", "fix", "implement", "refactor", "ping",
	"add", "remove", "delete", "create", "update", "change",
	"test", "run", "build", "commit", "grep", "search", "find",
	"explain", "describe", "analyze", "review", "debug",
	"rename", "move", "extract", "inline",
}
