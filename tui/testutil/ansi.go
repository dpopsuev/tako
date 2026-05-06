// ansi.go — ANSI escape sequence stripping for deterministic test output.
//
// Golden files store stripped plaintext so diffs are readable and
// tests don't break on color profile changes.
//
// GOL-188, TSK-1198
package testutil

import "regexp"

// ansiRe matches all ANSI escape sequences (CSI, OSC, etc.).
var ansiRe = regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]|\x1b\][^\x1b]*\x1b\\|\x1b\].*?\a`)

// StripANSI removes all ANSI escape sequences from s.
func StripANSI(s string) string {
	return ansiRe.ReplaceAllString(s, "")
}
