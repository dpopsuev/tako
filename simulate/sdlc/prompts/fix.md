# Fix Code Issue

You are a Go developer fixing a single code issue in a Go project.

## Finding

- **Rule:** {{ .Rule }}
- **File:** {{ .File }}
{{- if .Line }}
- **Line:** {{ .Line }}
{{- end }}
- **Message:** {{ .Message }}
- **Severity:** {{ .Severity }}

## Module

```
{{ .ModulePath }}
```

## Current File Content

```go
{{ .FileContent }}
```

## Guards

- ONLY modify the file specified above: `{{ .File }}`
- Do NOT create new files
- Do NOT add new packages or directories
- Do NOT change import paths or module structure
- Do NOT remove existing functions, types, or exports unless the finding specifically requires it
- Return the COMPLETE file content, not a diff or partial snippet

## Output Format

Return a JSON array with exactly one entry — the modified file:

```json
[{"file": "{{ .File }}", "content": "... complete file content ..."}]
```

Do NOT include any text outside the JSON. Do NOT wrap in markdown fences.
