package lsp

import (
	"go.lsp.dev/protocol"

	"github.com/dpopsuev/origami/lint"
)

func computeDiagnostics(doc *document) []protocol.Diagnostic {
	raw := []byte(doc.Content)
	if len(raw) == 0 {
		return nil
	}

	findings, err := lint.Run(raw, string(doc.URI), lint.WithProfile(lint.ProfileStrict))
	if err != nil {
		return []protocol.Diagnostic{{
			Range:    zeroRange(),
			Severity: protocol.DiagnosticSeverityError,
			Source:   "origami-lint",
			Message:  "Failed to parse circuit YAML: " + err.Error(),
		}}
	}

	diags := make([]protocol.Diagnostic, 0, len(findings))
	for i := range findings {
		diags = append(diags, findingToDiagnostic(&findings[i]))
	}
	return diags
}

func findingToDiagnostic(f *lint.Finding) protocol.Diagnostic {
	line := f.Line
	if line > 0 {
		line--
	}
	col := f.Column
	if col > 0 {
		col--
	}

	return protocol.Diagnostic{
		Range: protocol.Range{
			Start: protocol.Position{Line: safeUint32(line), Character: safeUint32(col)},
			End:   protocol.Position{Line: safeUint32(line), Character: safeUint32(col + 20)},
		},
		Severity: severityToLSP(f.Severity),
		Source:   "origami-lint",
		Code:     f.RuleID,
		Message:  f.Message,
	}
}

func severityToLSP(s lint.Severity) protocol.DiagnosticSeverity {
	switch s {
	case lint.SeverityError:
		return protocol.DiagnosticSeverityError
	case lint.SeverityWarning:
		return protocol.DiagnosticSeverityWarning
	case lint.SeverityInfo:
		return protocol.DiagnosticSeverityInformation
	default:
		return protocol.DiagnosticSeverityHint
	}
}

func zeroRange() protocol.Range {
	return protocol.Range{
		Start: protocol.Position{Line: 0, Character: 0},
		End:   protocol.Position{Line: 0, Character: 1},
	}
}
