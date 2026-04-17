package fold

import (
	"bytes"
	"fmt"
	"sort"
	"strings"
	"text/template"
)

const goNilLiteral = "nil"

// GenerateDomainServe produces Go source for a domain-serve binary that
// embeds domain files and serves them over MCP via domainserve.New().
// Supports both legacy (embed: directory) and assets (keyed file map) modes.
func GenerateDomainServe(m *Manifest) ([]byte, error) {
	if m.DomainServe == nil {
		return nil, ErrManifestHasNoDomainServeSection
	}
	ds := m.DomainServe
	if ds.Assets == nil {
		return nil, ErrDomainServeAssetsIsRequired
	}

	port := ds.Port
	if port == 0 {
		port = 9300
	}

	ctx := domainServeContext{
		Name:          m.Name,
		Version:       m.Version,
		Port:          port,
		EmbedPaths:    ds.Assets.AllPaths(),
		AssetSections: ds.Assets.Sections(),
		AssetFiles:    ds.Assets.ScalarFiles(),
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
	EmbedPaths    []string                     // assets mode: sorted file list
	AssetSections map[string]map[string]string // assets mode: section -> key -> path
	AssetFiles    map[string]string            // assets mode: singleton files
}

// goStringMap formats map[string]string as a Go literal.
func goStringMap(m map[string]string) string {
	if len(m) == 0 {
		return goNilLiteral
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
		return goNilLiteral
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
		return nil, ErrManifestHasNoDomainServeSection
	}
	ds := m.DomainServe

	port := ds.Port
	if port == 0 {
		port = 9300
	}

	needsOrigami := false
	for i := range g.Schematics {
		if g.Schematics[i].Resolver != "" {
			needsOrigami = true
			break
		}
	}

	needsFactory := g.Root.SessionFactory != ""

	needsTools := false
	for _, conn := range g.Connectors {
		if len(conn.Entries) > 0 {
			needsTools = true
			break
		}
	}

	stateDir := ""
	if ds.StateDir != "" {
		stateDir = ds.StateDir
	}

	ctx := wiredBinaryContext{
		Name:             m.Name,
		Version:          m.Version,
		Port:             port,
		StateDir:         stateDir,
		ImportBlock:      renderImports(g),
		WiringBlock:      renderWiring(g),
		DomainConfig:     renderDomainConfig(m),
		ServerBlock:      renderServerCreation(g, m.Name),
		InstrumentBlock:  renderInstrumentSetup(m),
		NeedsOrigami:     needsOrigami,
		NeedsFactory:     needsFactory,
		NeedsTools:       needsTools,
		NeedsInstruments: len(m.LoadedInstruments) > 0,
	}

	if ds.Assets != nil {
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
	Name             string
	Version          string
	Port             int
	StateDir         string
	EmbedPaths       []string
	ImportBlock      string
	WiringBlock      string
	DomainConfig     string
	ServerBlock      string
	InstrumentBlock  string
	NeedsOrigami     bool
	NeedsFactory     bool
	NeedsTools       bool
	NeedsInstruments bool
}

func renderImports(g *ResolvedGraph) string {
	var b strings.Builder
	for _, imp := range g.Imports {
		fmt.Fprintf(&b, "\t%s %q\n", imp.Alias, imp.Path)
	}
	return b.String()
}

func renderWiring(g *ResolvedGraph) string {
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

	for i := range g.Schematics {
		sch := &g.Schematics[i]
		if sch.Factory == "" {
			continue // resolver-only sub-schematic — no instance needed
		}
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
	b.WriteString("\tdomainHandler := domainserve.New(domainFS, domainserve.Config{\n")
	b.WriteString(fmt.Sprintf("\t\tName:    %q,\n", m.Name))
	b.WriteString(fmt.Sprintf("\t\tVersion: %q,\n", m.Version))

	if m.DomainServe != nil && m.DomainServe.Assets != nil {
		renderAssetIndex(&b, m.DomainServe.Assets)
	}
	b.WriteString("\t})\n")
	return b.String()
}

func renderAssetIndex(b *strings.Builder, a *AssetMap) {
	sections := a.Sections()
	files := a.ScalarFiles()
	if len(sections) == 0 && len(files) == 0 {
		return
	}
	b.WriteString("\t\tAssets: &domainserve.AssetIndex{\n")
	if len(sections) > 0 {
		fmt.Fprintf(b, "\t\t\tSections: %s,\n", goNestedMap(sections))
	}
	if len(files) > 0 {
		fmt.Fprintf(b, "\t\t\tFiles:    %s,\n", goStringMap(files))
	}
	b.WriteString("\t\t},\n")
}

func renderServerCreation(g *ResolvedGraph, productName string) string {
	root := g.Root

	// Factory mode: fold generates CircuitConfig inline, consumer provides only a SessionFactory.
	if root.SessionFactory != "" {
		return renderFactoryServer(g, productName)
	}

	// Legacy factory mode: consumer's NewServer handles everything.
	var b strings.Builder
	fmt.Fprintf(&b, "\tserver := %s.%s(%q,\n", root.Alias, root.Factory, productName)
	fmt.Fprintf(&b, "\t\t%s.WithDomainFS(domainFS),\n", root.Alias)
	for _, opt := range root.Options {
		fmt.Fprintf(&b, "\t\t%s.%s(%s),\n", root.Alias, opt.OptionFunc, opt.Provider)
	}

	var resolverEntries []string
	for i := range g.Schematics {
		if g.Schematics[i].Resolver != "" {
			resolverEntries = append(resolverEntries,
				fmt.Sprintf("\t\t\t%q: %s.%s(),", g.Schematics[i].Name, g.Schematics[i].Alias, g.Schematics[i].Resolver))
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

func renderFactoryServer(g *ResolvedGraph, productName string) string {
	var b strings.Builder
	root := g.Root

	// Get SessionFactory from consumer.
	fmt.Fprintf(&b, "\tfactory := %s.%s\n\n", root.Alias, root.SessionFactory)

	// Build ExtraParamDefs from component.yaml params.
	if len(root.Params) > 0 {
		b.WriteString("\textraParams := []fwmcp.ExtraParamDef{\n")
		for _, p := range root.Params {
			fmt.Fprintf(&b, "\t\t{Name: %q, Type: %q, Description: %q, Required: %v", p.Name, p.Type, p.Desc, p.Required)
			if len(p.Enum) > 0 {
				b.WriteString(", Enum: []string{")
				for i, e := range p.Enum {
					if i > 0 {
						b.WriteString(", ")
					}
					fmt.Fprintf(&b, "%q", e)
				}
				b.WriteString("}")
			}
			b.WriteString("},\n")
		}
		b.WriteString("\t}\n\n")
	}

	// Bridge SessionFactory → CircuitConfig.
	b.WriteString("\tbridgedCfg := fwmcp.SessionFactoryToConfig(factory)\n")
	fmt.Fprintf(&b, "\tbridgedCfg.Name = %q\n", productName)
	fmt.Fprintf(&b, "\tbridgedCfg.Version = %q\n", "1.0")
	b.WriteString("\tbridgedCfg.DomainFS = domainFS\n")
	b.WriteString("\tbridgedCfg.StateDir = *stateDir\n")
	b.WriteString("\tbridgedCfg.ResourceRegistry = fwresource.DefaultRegistry()\n")
	if len(root.Params) > 0 {
		b.WriteString("\tbridgedCfg.ExtraParamDefs = extraParams\n")
	}
	b.WriteString("\n")

	// Wire sub-circuit resolvers from sub-schematics.
	if len(g.Schematics) > 0 {
		var resolverEntries []string
		for i := range g.Schematics {
			if g.Schematics[i].Resolver != "" {
				resolverEntries = append(resolverEntries,
					fmt.Sprintf("\t\t%q: %s.%s(),", g.Schematics[i].Name, g.Schematics[i].Alias, g.Schematics[i].Resolver))
			}
		}
		if len(resolverEntries) > 0 {
			b.WriteString("\tbridgedCfg.SubCircuitResolvers = map[string]origami.AssetResolver{\n")
			for _, entry := range resolverEntries {
				fmt.Fprintf(&b, "%s\n", entry)
			}
			b.WriteString("\t}\n\n")
		}
	}

	// Build tools registry from connectors.
	if len(g.Connectors) > 0 {
		var toolEntries []string
		for _, conn := range g.Connectors {
			for _, entry := range conn.Entries {
				toolEntries = append(toolEntries,
					fmt.Sprintf("\ttools.Register(%s.%sTool())", conn.Alias, capitalize(entry.Socket)))
			}
		}
		if len(toolEntries) > 0 {
			b.WriteString("\ttools := fwtool.NewRegistry()\n")
			for _, entry := range toolEntries {
				fmt.Fprintf(&b, "%s\n", entry)
			}
			b.WriteString("\tbridgedCfg.Tools = tools\n\n")
		}
	}

	// Create server.
	b.WriteString("\tserver := fwmcp.NewCircuitServer(&bridgedCfg)\n")
	b.WriteString("\tdefer server.Shutdown()\n")

	return b.String()
}

// renderInstrumentSetup generates code that loads instrument manifests
// and builds an engine.ManifestRegistry. Called in the generated main().
func renderInstrumentSetup(m *Manifest) string {
	if len(m.LoadedInstruments) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("\t// Load instrument manifests.\n")
	b.WriteString("\tinstruments := fwengine.ManifestRegistry{}\n")
	for _, inst := range m.LoadedInstruments {
		fmt.Fprintf(&b, "\t{\n")
		fmt.Fprintf(&b, "\t\tm, err := fwdef.LoadInstrumentManifest(%q)\n", inst.Path)
		fmt.Fprintf(&b, "\t\tif err != nil {\n")
		fmt.Fprintf(&b, "\t\t\tfmt.Fprintf(os.Stderr, \"instrument %s: %%v\\n\", err)\n", inst.Name)
		fmt.Fprintf(&b, "\t\t\tos.Exit(1)\n")
		fmt.Fprintf(&b, "\t\t}\n")
		fmt.Fprintf(&b, "\t\tinstruments[%q] = m\n", inst.Name)
		fmt.Fprintf(&b, "\t}\n")
	}
	b.WriteString("\n")
	b.WriteString("\n")
	b.WriteString("\t// Preflight: verify all instruments are available.\n")
	b.WriteString("\tif err := fwengine.TuneAll(context.Background(), instruments, \"\"); err != nil {\n")
	b.WriteString("\t\tfmt.Fprintf(os.Stderr, \"instrument tune failed: %v\\n\", err)\n")
	b.WriteString("\t\tos.Exit(1)\n")
	b.WriteString("\t}\n\n")
	return b.String()
}

func capitalize(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

var wiredBinaryTemplate = `// Code generated by origami fold. DO NOT EDIT.
package main

import (
{{ if .NeedsInstruments }}	"context"
{{ end }}	"embed"
	"flag"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"

{{ if .NeedsOrigami }}	origami "github.com/dpopsuev/origami/circuit"
{{ end }}{{ if .NeedsFactory }}	fwmcp "github.com/dpopsuev/origami/mcp"
	fwresource "github.com/dpopsuev/origami/resource"
{{ end }}{{ if .NeedsTools }}	fwtool "github.com/dpopsuev/origami/tool"
{{ end }}{{ if .NeedsInstruments }}	fwdef "github.com/dpopsuev/origami/circuit/def"
	fwengine "github.com/dpopsuev/origami/engine"
{{ end }}	"github.com/dpopsuev/origami/domainserve"
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
{{ .ImportBlock }})

{{ range .EmbedPaths -}}
//go:embed {{ . }}
{{ end -}}
var domainData embed.FS

func main() {
	dataDir := flag.String("data-dir", "", "serve domain data from this directory instead of embedded assets")
	healthz := flag.Bool("healthz", false, "probe /healthz and exit")
	versionFlag := flag.Bool("version", false, "print version and exit")
	port := flag.Int("port", {{ .Port }}, "listen port")
	stateDir := flag.String("state-dir", {{ printf "%q" .StateDir }}, "persistent state directory")
	flag.Parse()

	// Default state directory to XDG_STATE_HOME/<name> when not explicitly set.
	if *stateDir == "" {
		xdg := os.Getenv("XDG_STATE_HOME")
		if xdg == "" {
			home, _ := os.UserHomeDir()
			xdg = filepath.Join(home, ".local", "state")
		}
		defaultDir := filepath.Join(xdg, {{ printf "%q" .Name }})
		stateDir = &defaultDir
	}

	if *versionFlag {
		fmt.Printf("%s %s (port %d)\n", {{ printf "%q" .Name }}, {{ printf "%q" .Version }}, *port)
		os.Exit(0)
	}

	if *healthz {
		resp, err := http.Get(fmt.Sprintf("http://localhost:%d/healthz", *port))
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

{{ .InstrumentBlock }}{{ .WiringBlock }}
{{ .DomainConfig }}
{{ .ServerBlock }}{{ if and .NeedsInstruments .NeedsFactory }}	bridgedCfg.Manifests = instruments
{{ end }}
	mux := http.NewServeMux()
	mux.Handle("/domain/", domainHandler)
	mcpHandler := sdkmcp.NewStreamableHTTPHandler(
		func(_ *http.Request) *sdkmcp.Server { return server.{{ if .NeedsFactory }}MCPServer{{ else }}CircuitServer.MCPServer{{ end }} },
		&sdkmcp.StreamableHTTPOptions{Stateless: false},
	)
	mux.Handle("/mcp", mcpHandler)
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(200) })

	addr := fmt.Sprintf(":%d", *port)
	fmt.Fprintf(os.Stderr, {{ printf "%q" .Name }}+" listening on %s (domain: /domain/, circuit: /mcp)\n", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
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

{{ range .EmbedPaths -}}
//go:embed {{ . }}
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
		Assets: &domainserve.AssetIndex{
			Sections: {{ goNestedMap .AssetSections }},
			Files:    {{ goStringMap .AssetFiles }},
		},
	})
	addr := fmt.Sprintf(":%d", {{ .Port }})
	fmt.Fprintf(os.Stderr, "domain-serve listening on %s\n", addr)
	if err := http.ListenAndServe(addr, handler); err != nil {
		fmt.Fprintf(os.Stderr, "domain-serve: %v\n", err)
		os.Exit(1)
	}
}
`
