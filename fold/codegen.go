package fold

import (
	"bytes"
	"fmt"
	"sort"
	"strings"
	"text/template"
)

// GenerateDomainServe produces Go source for a domain-serve binary that
// embeds domain files and serves them over MCP via domainserve.New().
// Supports both legacy (embed: directory) and assets (keyed file map) modes.
func GenerateDomainServe(m *Manifest) ([]byte, error) {
	if m.DomainServe == nil {
		return nil, fmt.Errorf("manifest has no domain_serve section")
	}
	ds := m.DomainServe
	if ds.Embed == "" && ds.Assets == nil {
		return nil, fmt.Errorf("domain_serve: one of embed or assets is required")
	}

	port := ds.Port
	if port == 0 {
		port = 9300
	}

	ctx := domainServeContext{
		Name:    m.Name,
		Version: m.Version,
		Port:    port,
	}

	if ds.Embed != "" {
		ctx.EmbedDir = strings.TrimRight(ds.Embed, "/")
	} else {
		ctx.EmbedPaths = ds.Assets.AllPaths()
		ctx.AssetSections = ds.Assets.Sections()
		ctx.AssetFiles = ds.Assets.ScalarFiles()
	}

	tmpl, err := template.New("domain-serve").Funcs(template.FuncMap{
		"goStringMap": goStringMap,
		"goNestedMap": goNestedMap,
	}).Parse(domainServeTemplate)
	if err != nil {
		return nil, fmt.Errorf("parse domain-serve template: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, ctx); err != nil {
		return nil, fmt.Errorf("execute domain-serve template: %w", err)
	}

	return buf.Bytes(), nil
}

type domainServeContext struct {
	Name          string
	Version       string
	Port          int
	EmbedDir      string                       // legacy mode
	EmbedPaths    []string                     // assets mode: sorted file list
	AssetSections map[string]map[string]string // assets mode: section -> key -> path
	AssetFiles    map[string]string            // assets mode: singleton files
}

func (c domainServeContext) HasAssets() bool {
	return len(c.EmbedPaths) > 0
}

// goStringMap formats map[string]string as a Go literal.
func goStringMap(m map[string]string) string {
	if len(m) == 0 {
		return "nil"
	}
	var b strings.Builder
	b.WriteString("map[string]string{")
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		fmt.Fprintf(&b, "%q: %q, ", k, m[k])
	}
	b.WriteString("}")
	return b.String()
}

// goNestedMap formats map[string]map[string]string as a Go literal.
func goNestedMap(m map[string]map[string]string) string {
	if len(m) == 0 {
		return "nil"
	}
	var b strings.Builder
	b.WriteString("map[string]map[string]string{\n")
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		fmt.Fprintf(&b, "\t\t\t%q: %s,\n", k, goStringMap(m[k]))
	}
	b.WriteString("\t\t}")
	return b.String()
}

// GenerateWiredBinary produces Go source for a unified binary that embeds
// domain files, wires connectors to schematics via resolved bindings, and
// serves both domain data and the circuit MCP server over Streamable HTTP.
func GenerateWiredBinary(m *Manifest, g *ResolvedGraph) ([]byte, error) {
	if m.DomainServe == nil {
		return nil, fmt.Errorf("manifest has no domain_serve section")
	}
	ds := m.DomainServe

	port := ds.Port
	if port == 0 {
		port = 9300
	}

	ctx := wiredBinaryContext{
		Name:         m.Name,
		Version:      m.Version,
		Port:         port,
		ImportBlock:  renderImports(g),
		WiringBlock:  renderWiring(g, m.Name),
		DomainConfig: renderDomainConfig(m),
		ServerBlock:  renderServerCreation(g, m.Name),
	}

	if ds.Embed != "" {
		ctx.EmbedDir = strings.TrimRight(ds.Embed, "/")
	} else if ds.Assets != nil {
		ctx.EmbedPaths = ds.Assets.AllPaths()
	}

	tmpl, err := template.New("wired-binary").Parse(wiredBinaryTemplate)
	if err != nil {
		return nil, fmt.Errorf("parse wired-binary template: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, ctx); err != nil {
		return nil, fmt.Errorf("execute wired-binary template: %w", err)
	}

	return buf.Bytes(), nil
}

type wiredBinaryContext struct {
	Name         string
	Version      string
	Port         int
	EmbedDir     string
	EmbedPaths   []string
	ImportBlock  string
	WiringBlock  string
	DomainConfig string
	ServerBlock  string
}

func (c wiredBinaryContext) HasAssets() bool {
	return len(c.EmbedPaths) > 0
}

func renderImports(g *ResolvedGraph) string {
	var b strings.Builder
	for _, imp := range g.Imports {
		fmt.Fprintf(&b, "\t%s %q\n", imp.Alias, imp.Path)
	}
	return b.String()
}

func renderWiring(g *ResolvedGraph, productName string) string {
	var b strings.Builder

	for _, conn := range g.Connectors {
		for _, entry := range conn.Entries {
			vn := varName(conn.Name, entry.Socket)
			fmt.Fprintf(&b, "\t%s, err := %s.%s()\n", vn, conn.Alias, entry.Factory)
			fmt.Fprintf(&b, "\tif err != nil {\n")
			fmt.Fprintf(&b, "\t\tfmt.Fprintf(os.Stderr, \"connector %s (%s): %%v\\n\", err)\n", conn.Name, entry.Socket)
			fmt.Fprintf(&b, "\t\tos.Exit(1)\n")
			fmt.Fprintf(&b, "\t}\n\n")
		}
	}

	for _, sch := range g.Schematics {
		fmt.Fprintf(&b, "\t%sInstance := %s.%s(\n", sch.Alias, sch.Alias, sch.Factory)
		for _, opt := range sch.Options {
			fmt.Fprintf(&b, "\t\t%s.%s(%s),\n", sch.Alias, opt.OptionFunc, opt.Provider)
		}
		fmt.Fprintf(&b, "\t)\n\n")
	}

	return b.String()
}

func renderDomainConfig(m *Manifest) string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("\tdomainHandler := domainserve.New(domainFS, domainserve.Config{\n"))
	b.WriteString(fmt.Sprintf("\t\tName:    %q,\n", m.Name))
	b.WriteString(fmt.Sprintf("\t\tVersion: %q,\n", m.Version))

	if m.DomainServe != nil && m.DomainServe.Assets != nil {
		a := m.DomainServe.Assets
		sections := a.Sections()
		files := a.ScalarFiles()
		if len(sections) > 0 || len(files) > 0 {
			b.WriteString("\t\tAssets: &domainserve.AssetIndex{\n")
			if len(sections) > 0 {
				b.WriteString(fmt.Sprintf("\t\t\tSections: %s,\n", goNestedMap(sections)))
			}
			if len(files) > 0 {
				b.WriteString(fmt.Sprintf("\t\t\tFiles:    %s,\n", goStringMap(files)))
			}
			b.WriteString("\t\t},\n")
		}
	}
	b.WriteString("\t})\n")
	return b.String()
}

func renderServerCreation(g *ResolvedGraph, productName string) string {
	var b strings.Builder
	root := g.Root

	fmt.Fprintf(&b, "\tserver := %s.%s(%q,\n", root.Alias, root.Factory, productName)
	fmt.Fprintf(&b, "\t\t%s.WithDomainFS(domainFS),\n", root.Alias)
	for _, opt := range root.Options {
		fmt.Fprintf(&b, "\t\t%s.%s(%s),\n", root.Alias, opt.OptionFunc, opt.Provider)
	}

	// Wire sub-circuit resolvers for schematics that declare a resolver function.
	// This enables overlay import resolution (e.g., circuits/harvester.yaml → dsr base).
	var resolverEntries []string
	for _, s := range g.Schematics {
		if s.Resolver != "" {
			resolverEntries = append(resolverEntries,
				fmt.Sprintf("\t\t\t%q: %s.%s(),", s.Name, s.Alias, s.Resolver))
		}
	}
	if len(resolverEntries) > 0 {
		fmt.Fprintf(&b, "\t\t%s.WithSubCircuitResolvers(map[string]origami.AssetResolver{\n", root.Alias)
		for _, entry := range resolverEntries {
			fmt.Fprintf(&b, "%s\n", entry)
		}
		fmt.Fprintf(&b, "\t\t}),\n")
	}

	fmt.Fprintf(&b, "\t)\n")
	return b.String()
}

var wiredBinaryTemplate = `// Code generated by origami fold. DO NOT EDIT.
package main

import (
	"embed"
	"flag"
	"fmt"
	"io/fs"
	"net/http"
	"os"

	origami "github.com/dpopsuev/origami"
	"github.com/dpopsuev/origami/domainserve"
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
{{ .ImportBlock }})

{{ if .HasAssets -}}
{{ range .EmbedPaths -}}
//go:embed {{ . }}
{{ end -}}
{{ else -}}
//go:embed {{ .EmbedDir }}
{{ end -}}
var domainData embed.FS

func main() {
	dataDir := flag.String("data-dir", "", "serve domain data from this directory instead of embedded assets")
	flag.Parse()

	var domainFS fs.FS = domainData
	if *dataDir != "" {
		domainFS = os.DirFS(*dataDir)
		fmt.Fprintf(os.Stderr, "using data dir: %s\n", *dataDir)
	}

{{ .WiringBlock }}
{{ .DomainConfig }}
{{ .ServerBlock }}
	mux := http.NewServeMux()
	mux.Handle("/domain/", domainHandler)
	mcpHandler := sdkmcp.NewStreamableHTTPHandler(
		func(_ *http.Request) *sdkmcp.Server { return server.CircuitServer.MCPServer },
		&sdkmcp.StreamableHTTPOptions{Stateless: false},
	)
	mux.Handle("/mcp", mcpHandler)
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(200) })

	addr := fmt.Sprintf(":%d", {{ .Port }})
	fmt.Fprintf(os.Stderr, {{ printf "%q" .Name }}+" listening on %s (domain: /domain/, circuit: /mcp)\n", addr)
	if err = http.ListenAndServe(addr, mux); err != nil {
		fmt.Fprintf(os.Stderr, {{ printf "%q" .Name }}+": %v\n", err)
		os.Exit(1)
	}
}
`

var domainServeTemplate = `// Code generated by origami fold. DO NOT EDIT.
package main

import (
	"embed"
	"flag"
	"fmt"
	"io/fs"
	"net/http"
	"os"

	"github.com/dpopsuev/origami/domainserve"
)

{{ if .HasAssets -}}
{{ range .EmbedPaths -}}
//go:embed {{ . }}
{{ end -}}
{{ else -}}
//go:embed {{ .EmbedDir }}
{{ end -}}
var domainData embed.FS

func main() {
	dataDir := flag.String("data-dir", "", "serve domain data from this directory instead of embedded assets")
	healthz := flag.Bool("healthz", false, "probe /healthz and exit")
	flag.Parse()

	if *healthz {
		resp, err := http.Get(fmt.Sprintf("http://localhost:%d/healthz", {{ .Port }}))
		if err != nil || resp.StatusCode != http.StatusOK {
			os.Exit(1)
		}
		os.Exit(0)
	}

	var domainFS fs.FS = domainData
	if *dataDir != "" {
		domainFS = os.DirFS(*dataDir)
		fmt.Fprintf(os.Stderr, "using data dir: %s\n", *dataDir)
	}

	handler := domainserve.New(domainFS, domainserve.Config{
		Name:    {{ printf "%q" .Name }},
		Version: {{ printf "%q" .Version }},
{{ if .HasAssets -}}
		Assets: &domainserve.AssetIndex{
			Sections: {{ goNestedMap .AssetSections }},
			Files:    {{ goStringMap .AssetFiles }},
		},
{{ end -}}
	})
	addr := fmt.Sprintf(":%d", {{ .Port }})
	fmt.Fprintf(os.Stderr, "domain-serve listening on %s\n", addr)
	if err := http.ListenAndServe(addr, handler); err != nil {
		fmt.Fprintf(os.Stderr, "domain-serve: %v\n", err)
		os.Exit(1)
	}
}
`
