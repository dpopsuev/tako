package lint

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/mattn/go-isatty"
)

// ANSI color codes for terminal output.
const (
	ansiReset  = "\033[0m"
	ansiBold   = "\033[1m"
	ansiRed    = "\033[31m"
	ansiYellow = "\033[33m"
	ansiCyan   = "\033[36m"
	ansiBlue   = "\033[34m"
)

// Renderer formats lint findings for terminal display.
// When writing to a TTY it uses ANSI colors; otherwise plain text.
type Renderer struct {
	w     io.Writer
	color bool
}

// NewRenderer creates a Renderer that writes to w.
// Color is enabled when w is a *os.File attached to a terminal.
func NewRenderer(w io.Writer) *Renderer {
	color := false
	if f, ok := w.(*os.File); ok {
		color = isatty.IsTerminal(f.Fd()) || isatty.IsCygwinTerminal(f.Fd())
	}
	return &Renderer{w: w, color: color}
}

// NewRendererWithColor creates a Renderer with explicit color control.
func NewRendererWithColor(w io.Writer, color bool) *Renderer {
	return &Renderer{w: w, color: color}
}

// Render formats a single Finding in rustc-style diagnostic format.
func (r *Renderer) Render(f Finding) {
	sev := f.Severity.String()
	sevColor := r.severityColor(f.Severity)

	// Header: error[S2]: invalid approach
	fmt.Fprintf(r.w, "%s%s[%s]%s: %s\n",
		sevColor, sev, f.RuleID, r.reset(), f.Message)

	// Location: --> circuits/alpha.yaml:15:5
	if f.File != "" {
		loc := f.File
		if f.Line > 0 {
			loc += fmt.Sprintf(":%d", f.Line)
			if f.Column > 0 {
				loc += fmt.Sprintf(":%d", f.Column)
			}
		}
		fmt.Fprintf(r.w, "  %s-->%s %s\n", r.blue(), r.reset(), loc)
	}

	// Expected/Found
	if f.Expected != "" || f.Found != "" {
		fmt.Fprintf(r.w, "   %s|%s\n", r.blue(), r.reset())
		if f.Expected != "" {
			fmt.Fprintf(r.w, "   %s=%s expected: %s\n", r.blue(), r.reset(), f.Expected)
		}
		if f.Found != "" {
			fmt.Fprintf(r.w, "   %s=%s    found: %s\n", r.blue(), r.reset(), f.Found)
		}
	}

	// Related spans
	for _, span := range f.Related {
		loc := span.File
		if span.Line > 0 {
			loc += fmt.Sprintf(":%d", span.Line)
		}
		label := span.Label
		if label == "" {
			label = "related"
		}
		fmt.Fprintf(r.w, "  %s-->%s %s (%s)\n", r.blue(), r.reset(), loc, label)
	}

	// Reason
	if f.Reason != "" {
		fmt.Fprintf(r.w, "   %s=%s reason: %s\n", r.blue(), r.reset(), f.Reason)
	}

	// Help text
	if f.HelpText != "" {
		fmt.Fprintf(r.w, "   %s=%s help: %s\n", r.blue(), r.reset(), f.HelpText)
	}

	// Suggestion (legacy field, still supported)
	if f.Suggestion != "" && f.HelpText == "" {
		fmt.Fprintf(r.w, "   %s=%s help: %s\n", r.blue(), r.reset(), f.Suggestion)
	}

	fmt.Fprintln(r.w)
}

// RenderAll formats all findings, returning the total count rendered.
func (r *Renderer) RenderAll(findings []Finding) int {
	for i := range findings {
		r.Render(findings[i])
	}
	if len(findings) > 0 {
		errs := 0
		warns := 0
		for i := range findings {
			switch findings[i].Severity {
			case SeverityError:
				errs++
			case SeverityWarning:
				warns++
			}
		}
		var parts []string
		if errs > 0 {
			parts = append(parts, fmt.Sprintf("%d error(s)", errs))
		}
		if warns > 0 {
			parts = append(parts, fmt.Sprintf("%d warning(s)", warns))
		}
		if len(parts) > 0 {
			fmt.Fprintf(r.w, "%s%s%s\n", r.severityColor(findings[0].Severity),
				strings.Join(parts, ", "), r.reset())
		}
	}
	return len(findings)
}

func (r *Renderer) severityColor(s Severity) string {
	if !r.color {
		return ""
	}
	switch s {
	case SeverityError:
		return ansiBold + ansiRed
	case SeverityWarning:
		return ansiBold + ansiYellow
	case SeverityInfo:
		return ansiBold + ansiCyan
	default:
		return ""
	}
}

func (r *Renderer) blue() string {
	if !r.color {
		return ""
	}
	return ansiBlue
}

func (r *Renderer) reset() string {
	if !r.color {
		return ""
	}
	return ansiReset
}
