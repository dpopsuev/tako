package fold

import (
	"context"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// ModuleResolver locates Go modules on the local filesystem.
// The default implementation searches $HOME/Workspace and ./
// but callers can supply custom resolvers for CI or non-standard layouts.
type ModuleResolver interface {
	FindLocalModule(modPath string) string
}

// DefaultModuleResolver searches for Go modules in well-known locations.
type DefaultModuleResolver struct {
	ExtraDirs []string
}

func (r *DefaultModuleResolver) FindLocalModule(modPath string) string {
	home, _ := os.UserHomeDir()
	candidates := make([]string, 0, 2+len(r.ExtraDirs))
	if home != "" {
		candidates = append(candidates, filepath.Join(home, "Workspace", filepath.Base(modPath)))
	}
	candidates = append(candidates, filepath.Join(".", filepath.Base(modPath)))
	for _, d := range r.ExtraDirs {
		candidates = append(candidates, filepath.Join(d, filepath.Base(modPath)))
	}
	for _, c := range candidates {
		if _, err := os.Stat(filepath.Join(c, "go.mod")); err == nil {
			abs, _ := filepath.Abs(c)
			return abs
		}
	}
	return ""
}

// Options configures the fold build.
type Options struct {
	ManifestPath   string
	Output         string
	GoFlags        []string
	Verbose        bool
	Container      bool // build an OCI image instead of a local binary
	DomainOnly     bool // force domain-serve build even when schematics are declared
	ImageName      string
	ModuleResolver ModuleResolver
}

// Run loads the manifest, generates the appropriate binary, and compiles it.
// When schematics are declared, it produces a unified wired binary with
// connector binding. Otherwise it produces a domain-serve-only binary.
// The context controls cancellation and deadlines for all subprocess calls
// (go mod tidy, go build, docker build).
func Run(ctx context.Context, opts Options) error {
	m, err := LoadManifest(opts.ManifestPath)
	if err != nil {
		return err
	}

	if m.DomainServe == nil {
		return fmt.Errorf("manifest must have a domain_serve section")
	}

	manifestDir := filepath.Dir(opts.ManifestPath)
	if err := validateManifest(m, manifestDir, opts.Verbose); err != nil {
		return err
	}

	if m.HasBindings() && !opts.DomainOnly {
		return buildWiredBinary(ctx, m, opts)
	}
	return buildDomainServe(ctx, m, opts)
}

// validateManifest runs manifest-level checks: domain directories, duplicate domains,
// and output_schema path resolution.
func validateManifest(m *Manifest, manifestDir string, verbose bool) error {
	if err := validateNoDuplicateDomains(m); err != nil {
		return err
	}
	if err := validateDomainDirs(m, manifestDir, verbose); err != nil {
		return err
	}
	if err := validateAssetPaths(m, manifestDir); err != nil {
		return err
	}
	return validateCircuitRefs(m, manifestDir)
}

func validateNoDuplicateDomains(m *Manifest) error {
	seen := make(map[string]bool)
	for _, d := range m.Domains {
		if seen[d] {
			return fmt.Errorf("manifest: duplicate domain %q", d)
		}
		seen[d] = true
	}
	return nil
}

func validateDomainDirs(m *Manifest, manifestDir string, verbose bool) error {
	for _, d := range m.Domains {
		dir := filepath.Join(manifestDir, "domains", d)
		if info, err := os.Stat(dir); err != nil || !info.IsDir() {
			return fmt.Errorf("domain %q declared in manifest but domains/%s/ not found", d, d)
		}
	}

	domainsRoot := filepath.Join(manifestDir, "domains")
	if _, err := os.Stat(domainsRoot); err != nil {
		return nil
	}

	declared := make(map[string]bool)
	for _, d := range m.Domains {
		declared[d] = true
	}

	entries, _ := os.ReadDir(domainsRoot)
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		subEntries, _ := os.ReadDir(filepath.Join(domainsRoot, e.Name()))
		for _, sub := range subEntries {
			if !sub.IsDir() {
				continue
			}
			path := e.Name() + "/" + sub.Name()
			if !declared[path] && verbose {
				fmt.Fprintf(os.Stderr, "warning: domains/%s/ exists but is not in manifest domains: list\n", path)
			}
		}
	}
	return nil
}

func validateAssetPaths(m *Manifest, manifestDir string) error {
	if m.DomainServe == nil || m.DomainServe.Assets == nil {
		return nil
	}
	paths := m.DomainServe.Assets.AllPaths()
	if m.DomainServe.Store != nil && m.DomainServe.Store.Schema != "" {
		paths = append(paths, m.DomainServe.Store.Schema)
	}
	for _, p := range paths {
		full := filepath.Join(manifestDir, p)
		if _, err := os.Stat(full); err != nil {
			return fmt.Errorf("asset path %q not found on disk", p)
		}
	}
	return nil
}

const (
	origamiModule = "github.com/dpopsuev/origami"
	mcpSDKModule  = "github.com/modelcontextprotocol/go-sdk"
)

func buildWiredBinary(ctx context.Context, m *Manifest, opts Options) error {
	resolver := opts.ModuleResolver
	if resolver == nil {
		resolver = &DefaultModuleResolver{}
	}

	origamiRoot := resolver.FindLocalModule(origamiModule)
	if origamiRoot == "" {
		return fmt.Errorf("cannot find origami module on local filesystem")
	}

	manifestDir := filepath.Dir(opts.ManifestPath)
	if err := m.MergeDiscoveredAssets(manifestDir); err != nil {
		return fmt.Errorf("discover domain assets: %w", err)
	}

	g, err := Resolve(m, origamiRoot, resolver)
	if err != nil {
		return fmt.Errorf("resolve bindings: %w", err)
	}

	src, err := GenerateWiredBinary(m, g)
	if err != nil {
		return err
	}

	tmpDir, err := os.MkdirTemp("", "origami-fold-wired-*")
	if err != nil {
		return fmt.Errorf("create temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	if err := os.WriteFile(filepath.Join(tmpDir, "main.go"), src, 0644); err != nil {
		return fmt.Errorf("write main.go: %w", err)
	}

	if opts.Verbose {
		fmt.Fprintf(os.Stderr, "wired binary generated main.go (%d bytes)\n", len(src))
		fmt.Fprintf(os.Stderr, "%s\n", string(src))
	}

	if err := copyDomainFiles(m, manifestDir, tmpDir, opts.Verbose); err != nil {
		return err
	}
	if err := copyEmbedFiles(m.DomainServe, manifestDir, tmpDir, opts.Verbose); err != nil {
		return err
	}

	if err := createWiredBuildModule(tmpDir, m.Name, resolver, g); err != nil {
		return fmt.Errorf("create build module: %w", err)
	}

	tidy := exec.CommandContext(ctx, "go", "mod", "tidy")
	tidy.Dir = tmpDir
	tidy.Stdout = os.Stdout
	tidy.Stderr = os.Stderr
	tidy.Env = os.Environ()
	if err := tidy.Run(); err != nil {
		return fmt.Errorf("go mod tidy: %w", err)
	}

	output := opts.Output
	if output == "" {
		output = filepath.Join("bin", m.Name)
	}
	if !filepath.IsAbs(output) {
		wd, _ := os.Getwd()
		output = filepath.Join(wd, output)
	}

	if err := os.MkdirAll(filepath.Dir(output), 0755); err != nil {
		return fmt.Errorf("create output dir: %w", err)
	}

	args := []string{"build", "-o", output}
	args = append(args, opts.GoFlags...)
	args = append(args, ".")

	cmd := exec.CommandContext(ctx, "go", args...)
	cmd.Dir = tmpDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = os.Environ()

	if opts.Verbose {
		fmt.Fprintf(os.Stderr, "running: go %s (in %s)\n", strings.Join(args, " "), tmpDir)
	}

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("go build wired binary: %w", err)
	}

	fmt.Fprintf(os.Stderr, "built %s\n", output)
	return nil
}

func createWiredBuildModule(tmpDir, name string, resolver ModuleResolver, g *ResolvedGraph) error {
	var buf strings.Builder
	buf.WriteString(fmt.Sprintf("module %s-build\n\ngo 1.24\n\nrequire (\n", name))
	buf.WriteString(fmt.Sprintf("\t%s v0.0.0\n", origamiModule))
	buf.WriteString(fmt.Sprintf("\t%s v0.0.0\n", mcpSDKModule))

	// Collect unique external module roots from resolved imports.
	seen := map[string]bool{origamiModule: true, mcpSDKModule: true}
	var externalModules []string
	if g != nil {
		for _, imp := range g.Imports {
			mod := moduleRoot(imp.Path)
			if mod != "" && !seen[mod] {
				seen[mod] = true
				externalModules = append(externalModules, mod)
				buf.WriteString(fmt.Sprintf("\t%s v0.0.0\n", mod))
			}
		}
	}
	buf.WriteString(")\n\n")

	// Add replace directives for locally available modules.
	if localPath := resolver.FindLocalModule(origamiModule); localPath != "" {
		buf.WriteString(fmt.Sprintf("replace %s => %s\n", origamiModule, localPath))
	}
	for _, mod := range externalModules {
		if localPath := resolver.FindLocalModule(mod); localPath != "" {
			buf.WriteString(fmt.Sprintf("replace %s => %s\n", mod, localPath))
		}
	}

	return os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte(buf.String()), 0644)
}

// moduleRoot extracts the module root from a Go import path.
// "github.com/dpopsuev/rh-rca/connectors/rp" → "github.com/dpopsuev/rh-rca"
// Returns "" for standard library or origami-internal paths.
func moduleRoot(importPath string) string {
	parts := strings.Split(importPath, "/")
	if len(parts) < 3 || !strings.Contains(parts[0], ".") {
		return ""
	}
	return strings.Join(parts[:3], "/")
}

func buildDomainServe(ctx context.Context, m *Manifest, opts Options) error {
	manifestDir := filepath.Dir(opts.ManifestPath)
	if err := m.MergeDiscoveredAssets(manifestDir); err != nil {
		return fmt.Errorf("discover domain assets: %w", err)
	}

	src, err := GenerateDomainServe(m)
	if err != nil {
		return err
	}

	tmpDir, err := os.MkdirTemp("", "origami-fold-domain-*")
	if err != nil {
		return fmt.Errorf("create temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	if err := os.WriteFile(filepath.Join(tmpDir, "main.go"), src, 0644); err != nil {
		return fmt.Errorf("write main.go: %w", err)
	}

	if opts.Verbose {
		fmt.Fprintf(os.Stderr, "domain-serve generated main.go (%d bytes)\n", len(src))
		fmt.Fprintf(os.Stderr, "%s\n", string(src))
	}

	if err := copyDomainFiles(m, manifestDir, tmpDir, opts.Verbose); err != nil {
		return err
	}
	if err := copyEmbedFiles(m.DomainServe, manifestDir, tmpDir, opts.Verbose); err != nil {
		return err
	}

	resolver := opts.ModuleResolver
	if resolver == nil {
		resolver = &DefaultModuleResolver{}
	}

	if err := createDomainServeBuildModule(tmpDir, m.Name, resolver); err != nil {
		return fmt.Errorf("create build module: %w", err)
	}

	tidy := exec.CommandContext(ctx, "go", "mod", "tidy")
	tidy.Dir = tmpDir
	tidy.Stdout = os.Stdout
	tidy.Stderr = os.Stderr
	tidy.Env = os.Environ()
	if err := tidy.Run(); err != nil {
		return fmt.Errorf("go mod tidy: %w", err)
	}

	output := opts.Output
	if output == "" {
		output = filepath.Join("bin", m.Name+"-domain-serve")
	}
	if !filepath.IsAbs(output) {
		wd, _ := os.Getwd()
		output = filepath.Join(wd, output)
	}

	if err := os.MkdirAll(filepath.Dir(output), 0755); err != nil {
		return fmt.Errorf("create output dir: %w", err)
	}

	args := []string{"build", "-o", output}
	args = append(args, opts.GoFlags...)
	args = append(args, ".")

	cmd := exec.CommandContext(ctx, "go", args...)
	cmd.Dir = tmpDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	env := os.Environ()
	if opts.Container {
		env = append(env, "CGO_ENABLED=0")
	}
	cmd.Env = env

	if opts.Verbose {
		fmt.Fprintf(os.Stderr, "running: go %s (in %s)\n", strings.Join(args, " "), tmpDir)
	}

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("go build domain-serve: %w", err)
	}

	fmt.Fprintf(os.Stderr, "built %s\n", output)

	if opts.Container {
		return buildContainerImage(ctx, m, output, opts)
	}
	return nil
}

const containerDockerfileTemplate = `FROM gcr.io/distroless/static-debian12
COPY domain-serve /domain-serve
ENTRYPOINT ["/domain-serve"]
EXPOSE %d
`

func buildContainerImage(ctx context.Context, m *Manifest, binaryPath string, opts Options) error {
	port := 9300
	if m.DomainServe != nil && m.DomainServe.Port != 0 {
		port = m.DomainServe.Port
	}

	imgName := opts.ImageName
	if imgName == "" {
		imgName = "origami-" + m.Name + "-domain"
	}

	imgDir, err := os.MkdirTemp("", "origami-fold-image-*")
	if err != nil {
		return fmt.Errorf("create image dir: %w", err)
	}
	defer os.RemoveAll(imgDir)

	dockerfile := fmt.Sprintf(containerDockerfileTemplate, port)
	if err := os.WriteFile(filepath.Join(imgDir, "Dockerfile"), []byte(dockerfile), 0644); err != nil {
		return fmt.Errorf("write Dockerfile: %w", err)
	}

	src, err := os.Open(binaryPath)
	if err != nil {
		return fmt.Errorf("open binary: %w", err)
	}
	defer src.Close()

	dst, err := os.OpenFile(filepath.Join(imgDir, "domain-serve"), os.O_CREATE|os.O_WRONLY, 0755)
	if err != nil {
		return fmt.Errorf("copy binary: %w", err)
	}
	if _, err := io.Copy(dst, src); err != nil {
		dst.Close()
		return fmt.Errorf("copy binary: %w", err)
	}
	dst.Close()

	dockerCmd := exec.CommandContext(ctx, "docker", "build", "-t", imgName, ".")
	dockerCmd.Dir = imgDir
	dockerCmd.Stdout = os.Stdout
	dockerCmd.Stderr = os.Stderr

	if opts.Verbose {
		fmt.Fprintf(os.Stderr, "running: docker build -t %s . (in %s)\n", imgName, imgDir)
	}

	if err := dockerCmd.Run(); err != nil {
		return fmt.Errorf("docker build: %w", err)
	}

	fmt.Fprintf(os.Stderr, "built image %s\n", imgName)
	return nil
}

func createDomainServeBuildModule(tmpDir, name string, resolver ModuleResolver) error {
	var buf strings.Builder
	buf.WriteString(fmt.Sprintf("module %s-domain-serve-build\n\ngo 1.24\n\nrequire (\n", name))
	buf.WriteString(fmt.Sprintf("\t%s v0.0.0\n", origamiModule))
	buf.WriteString(")\n\n")

	localPath := resolver.FindLocalModule(origamiModule)
	if localPath != "" {
		buf.WriteString(fmt.Sprintf("replace %s => %s\n", origamiModule, localPath))
	}

	return os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte(buf.String()), 0644)
}

// copyDomainFiles copies files from domains/<path>/ into the build dir at
// flat paths expected by the runtime (e.g., domains/ocp/ptp/scenarios/x.yaml -> scenarios/x.yaml).
func copyDomainFiles(m *Manifest, manifestDir, tmpDir string, verbose bool) error {
	mappings := m.domainPathMappings(manifestDir)
	if len(mappings) == 0 {
		return nil
	}
	for src, flatDst := range mappings {
		dst := filepath.Join(tmpDir, flatDst)
		if err := copyFile(src, dst); err != nil {
			return fmt.Errorf("copy domain file %q: %w", flatDst, err)
		}
	}
	if verbose {
		fmt.Fprintf(os.Stderr, "copied %d domain files (flattened)\n", len(mappings))
	}
	return nil
}

func copyEmbedFiles(ds *DomainServeConfig, manifestDir, tmpDir string, verbose bool) error {
	if ds.Embed != "" {
		embedDir := strings.TrimRight(ds.Embed, "/")
		srcEmbed := filepath.Join(manifestDir, embedDir)
		dstEmbed := filepath.Join(tmpDir, embedDir)
		if err := copyDir(srcEmbed, dstEmbed); err != nil {
			return fmt.Errorf("copy embed dir %q: %w", embedDir, err)
		}
		if verbose {
			fmt.Fprintf(os.Stderr, "copied embed dir: %s -> %s\n", srcEmbed, dstEmbed)
		}
		return nil
	}

	paths := ds.Assets.AllPaths()
	if ds.Store != nil && ds.Store.Schema != "" {
		paths = append(paths, ds.Store.Schema)
	}
	for _, p := range paths {
		dstPath := filepath.Join(tmpDir, p)
		if _, err := os.Stat(dstPath); err == nil {
			continue // already placed by copyDomainFiles
		}
		srcPath := filepath.Join(manifestDir, p)
		if err := copyFile(srcPath, dstPath); err != nil {
			return fmt.Errorf("copy asset %q: %w", p, err)
		}
	}
	if verbose {
		fmt.Fprintf(os.Stderr, "copied %d asset files\n", len(paths))
	}
	return nil
}

// validateCircuitRefs checks that every node with handler_type: circuit
// references a circuit name that exists in assets.circuits, and that the
// circuit dependency graph is acyclic.
func validateCircuitRefs(m *Manifest, manifestDir string) error {
	if m.DomainServe == nil || m.DomainServe.Assets == nil {
		return nil
	}
	circuits := m.DomainServe.Assets.Circuits
	if len(circuits) == 0 {
		return nil
	}

	deps := make(map[string][]string)
	for name, path := range circuits {
		refs, err := extractCircuitRefs(filepath.Join(manifestDir, path))
		if err != nil {
			return fmt.Errorf("circuit %q: %w", name, err)
		}
		for _, ref := range refs {
			if _, ok := circuits[ref]; !ok {
				return fmt.Errorf("circuit %q references circuit %q which is not in assets.circuits", name, ref)
			}
		}
		deps[name] = refs
	}

	if cycle := detectCircuitCycle(deps); cycle != "" {
		return fmt.Errorf("circuit dependency cycle detected: %s", cycle)
	}
	return nil
}

type circuitFileForValidation struct {
	Nodes []struct {
		Name        string `yaml:"name"`
		HandlerType string `yaml:"handler_type"`
		Handler     string `yaml:"handler"`
	} `yaml:"nodes"`
}

func extractCircuitRefs(path string) ([]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read circuit: %w", err)
	}
	var cf circuitFileForValidation
	if err := yaml.Unmarshal(data, &cf); err != nil {
		return nil, fmt.Errorf("parse circuit: %w", err)
	}
	var refs []string
	for _, n := range cf.Nodes {
		if n.HandlerType == "circuit" && n.Handler != "" {
			refs = append(refs, n.Handler)
		}
	}
	return refs, nil
}

func detectCircuitCycle(deps map[string][]string) string {
	const (
		white = 0
		gray  = 1
		black = 2
	)
	color := make(map[string]int)
	var path []string

	var visit func(string) bool
	visit = func(node string) bool {
		color[node] = gray
		path = append(path, node)
		for _, dep := range deps[node] {
			switch color[dep] {
			case gray:
				path = append(path, dep)
				return true
			case white:
				if visit(dep) {
					return true
				}
			}
		}
		path = path[:len(path)-1]
		color[node] = black
		return false
	}

	sorted := make([]string, 0, len(deps))
	for k := range deps {
		sorted = append(sorted, k)
	}
	sort.Strings(sorted)

	for _, node := range sorted {
		if color[node] == white {
			if visit(node) {
				return strings.Join(path, " → ")
			}
		}
	}
	return ""
}

func copyFile(src, dst string) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()
	dstFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer dstFile.Close()
	_, err = io.Copy(dstFile, srcFile)
	return err
}

func copyDir(src, dst string) error {
	return filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)

		if d.IsDir() {
			return os.MkdirAll(target, 0755)
		}

		srcFile, err := os.Open(path)
		if err != nil {
			return err
		}
		defer srcFile.Close()

		if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
			return err
		}

		dstFile, err := os.Create(target)
		if err != nil {
			return err
		}
		defer dstFile.Close()

		_, err = io.Copy(dstFile, srcFile)
		return err
	})
}
