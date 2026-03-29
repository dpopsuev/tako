package lint

import (
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"
)

const ruleTemplateParam = "P1/template-param-validity"

// --- P1: template-param-validity ---

// TemplateParamValidity checks that prompt templates reference only valid
// parameter fields.  It walks the PromptFS for .md files containing Go
// template directives and delegates field validation to the PromptValidator
// callback (provided by the domain module via WithPromptValidator).
//
// When PromptFS or PromptValidator is nil the rule is a silent no-op.
type TemplateParamValidity struct{}

func (r *TemplateParamValidity) ID() string { return ruleTemplateParam }
func (r *TemplateParamValidity) Description() string {
	return "prompt template references invalid parameter field"
}
func (r *TemplateParamValidity) Severity() Severity { return SeverityWarning }
func (r *TemplateParamValidity) Tags() []string     { return []string{"prompt"} }

func (r *TemplateParamValidity) Check(ctx *LintContext) []Finding {
	if ctx.PromptFS == nil || ctx.PromptValidator == nil {
		return nil
	}

	var out []Finding
	_ = fs.WalkDir(ctx.PromptFS, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || filepath.Ext(path) != ".md" {
			return nil
		}
		data, readErr := fs.ReadFile(ctx.PromptFS, path)
		if readErr != nil {
			return nil
		}
		content := string(data)
		if !strings.Contains(content, "{{") {
			return nil
		}
		for _, fe := range ctx.PromptValidator(content) {
			if fe.Field == "" {
				continue
			}
			out = append(out, Finding{
				RuleID:   r.ID(),
				Severity: r.Severity(),
				Message:  fmt.Sprintf("template %s: field .%s — %s", path, fe.Field, fe.Message),
				File:     ctx.File,
			})
		}
		return nil
	})
	return out
}
