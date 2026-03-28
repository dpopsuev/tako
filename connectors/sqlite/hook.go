package sqlite

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"text/template"

	fw "github.com/dpopsuev/origami/circuit"
)

// ExecHook is a framework Hook that executes SQL statements on step completion.
// The query and parameters are read from NodeConfig and support Go template
// variables: {{ .NodeName }}, {{ .ArtifactField }}.
//
// Usage in circuit YAML:
//
//	nodes:
//	  - name: recall
//	    after: [sqlite-exec]
//	    meta:
//	      sqlite_query: "UPDATE cases SET symptom_id = ? WHERE id = ?"
//	      sqlite_params: ["{{ .symptom_id }}", "{{ .case_id }}"]
type ExecHook struct {
	db          *DB
	nodeConfigs map[string]*fw.NodeConfig
}

// NewExecHook creates a Hook that executes SQL via the given DB.
// nodeConfigs maps node names to their NodeConfig.
func NewExecHook(db *DB, nodeConfigs map[string]*fw.NodeConfig) *ExecHook {
	return &ExecHook{db: db, nodeConfigs: nodeConfigs}
}

const BuiltinHookSQLiteExec = "sqlite-exec"

func (h *ExecHook) Name() string { return BuiltinHookSQLiteExec }

func (h *ExecHook) Run(_ context.Context, nodeName string, artifact fw.Artifact) error {
	cfg := h.nodeConfigs[nodeName]
	if cfg == nil {
		return nil
	}

	query := cfg.SQLiteQuery
	if query == "" {
		return nil
	}

	raw := artifact.Raw()
	artMap := toMap(raw)

	artMap["NodeName"] = nodeName

	resolvedQuery, err := renderTemplate("query", query, artMap)
	if err != nil {
		return fmt.Errorf("sqlite-exec hook: render query: %w", err)
	}

	args := make([]any, 0, len(cfg.SQLiteParams))
	for i, pt := range cfg.SQLiteParams {
		s, ok := pt.(string)
		if !ok {
			args = append(args, pt)
			continue
		}
		resolved, err := renderTemplate(fmt.Sprintf("param[%d]", i), s, artMap)
		if err != nil {
			return fmt.Errorf("sqlite-exec hook: render param %d: %w", i, err)
		}
		args = append(args, resolved)
	}

	_, err = h.db.Exec(resolvedQuery, args...)
	if err != nil {
		return fmt.Errorf("sqlite-exec hook: exec %q: %w", resolvedQuery, err)
	}

	return nil
}

func renderTemplate(name, tmpl string, data map[string]any) (string, error) {
	if !strings.Contains(tmpl, "{{") {
		return tmpl, nil
	}
	t, err := template.New(name).Option("missingkey=zero").Parse(tmpl)
	if err != nil {
		return "", err
	}
	var buf strings.Builder
	if err := t.Execute(&buf, data); err != nil {
		return "", err
	}
	return buf.String(), nil
}

func toMap(v any) map[string]any {
	if m, ok := v.(map[string]any); ok {
		return m
	}
	data, err := json.Marshal(v)
	if err != nil {
		return map[string]any{"raw": v}
	}
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		return map[string]any{"raw": v}
	}
	return m
}
