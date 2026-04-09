package fold

import (
	"context"
	"fmt"
	"io"
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
	ExportDataDir  string // export flattened domain data to this directory (for volume mounts)
	Local          bool   // use local module overrides via replace directives (dev only)
	ModuleResolver ModuleResolver
}

// Run loads the manifest, generates the appropriate binary, and compiles it.
// Supports two manifest formats:
//   - Flat board (kind: Board) — new format, read directly
//   - K8s-style (kind: Board with apiVersion/metadata/spec) — legacy
//
// When schematics are declared, it produces a unified wired binary with
// connector binding. Otherwise it produces a domain-serve-only binary.
func Run(ctx context.Context, opts *Options) error {
	data, err := os.ReadFile(opts.ManifestPath)
	if err != nil {
		return fmt.Errorf("read manifest: %w", err)
	}

	// Detect format: K8s-style (has apiVersion field) vs flat board.
	var probe struct {
		APIVersion string `yaml:"apiVersion"`
	}
	if err := yaml.Unmarshal(data, &probe); err != nil {
		return fmt.Errorf("parse manifest envelope: %w", err)
	}

	if probe.APIVersion != "" {
		// K8s-style manifest.
		return runLegacy(ctx, data, opts)
	}

	// Flat board manifest.
	return runBoard(ctx, data, opts)
}

// runBoard handles the new flat board manifest format.
func runBoard(ctx context.Context, data []byte, opts *Options) error {
	bm, err := ParseBoardManifest(data)
	if err != nil {
		return err
	}

	// All paths in board are relative to CWD (repo root), not board file.
	// Override ManifestPath so downstream codegen (buildDomainServe, buildWiredBinary)
	// resolves paths from CWD, not from the boards/ subdirectory.
	baseDir, _ := os.Getwd()
	boardDir := filepath.Dir(opts.ManifestPath)
	opts.ManifestPath = filepath.Join(baseDir, filepath.Base(opts.ManifestPath))

	// Resolve composition — compose base paths are relative to board file.
	bm, err = ResolveBoardComposition(bm, boardDir)
	if err != nil {
		return err
	}

	// Validate kind at referenced paths (relative to CWD).
	if err := ValidateBoardPaths(bm, baseDir); err != nil {
		return err
	}

	// Bridge to legacy Manifest for codegen compatibility.
	m := boardToManifest(bm)

	// Domain file discovery + validation (relative to CWD).
	if err := m.MergeDiscoveredAssets(baseDir); err != nil {
		return err
	}
	if err := ValidateDomainKinds(m, baseDir); err != nil {
		return err
	}

	// Board-specific validation (replaces legacy validateManifest).
	// ValidateBoardPaths + ValidateDomainKinds already ran above.
	// Only run duplicate domain check from legacy — other validations
	// don't apply (asset paths are board-relative, not manifest-relative).
	if err := validateNoDuplicateDomains(m); err != nil {
		return err
	}

	if err := validatePortWiring(m, baseDir); err != nil {
		return err
	}

	if opts.ExportDataDir != "" {
		return exportDataDir(m, boardDir, opts)
	}

	if m.HasBindings() && !opts.DomainOnly {
		return buildWiredBinary(ctx, m, opts)
	}
	return buildDomainServe(ctx, m, opts)
}

// runLegacy handles the K8s-style origami.yaml manifest format.
func runLegacy(ctx context.Context, data []byte, opts *Options) error {
	m, err := ParseManifest(data)
	if err != nil {
		return err
	}

	if m.DomainServe == nil {
		return ErrManifestMustHaveADomainServeSection
	}

	manifestDir := filepath.Dir(opts.ManifestPath)
	if err := validateManifest(m, manifestDir, opts.Verbose); err != nil {
		return err
	}

	if opts.ExportDataDir != "" {
		return exportDataDir(m, manifestDir, opts)
	}

	if m.HasBindings() && !opts.DomainOnly {
		return buildWiredBinary(ctx, m, opts)
	}
	return buildDomainServe(ctx, m, opts)
}

// boardToManifest bridges BoardManifest to the legacy Manifest type
// so existing codegen (buildWiredBinary, buildDomainServe) works unchanged.
func boardToManifest(bm *BoardManifest) *Manifest {
	m := &Manifest{
		APIVersion:  "origami/v1",
		Kind:        "Board",
		Name:        bm.Name,
		Description: bm.Description,
		Domains:     []string{},
	}

	// Map domain path to domains list.
	// Board uses relative path (../domains/ocp/ptp), legacy expects name (ocp/ptp).
	// Strip the leading path to get just the domain name.
	if bm.Domain != "" {
		domain := bm.Domain
		// Strip ../domains/ or domains/ prefix if present.
		for _, prefix := range []string{"../domains/", "domains/", "./domains/"} {
			if strings.HasPrefix(domain, prefix) {
				domain = strings.TrimPrefix(domain, prefix)
				break
			}
		}
		m.Domains = append(m.Domains, domain)
	}

	// Map uses to legacy format.
	if len(bm.Uses) > 0 {
		m.Uses = make(map[string]UsesRef)
		for name, module := range bm.Uses {
			m.Uses[name] = UsesRef{Kind: "schematic", Module: module}
		}
	}

	// Map bind to legacy format.
	if len(bm.Bind) > 0 {
		m.Bind = make(map[string]map[string]string)
		for key, module := range bm.Bind {
			// key is "schematic.socket" (e.g., "rca.source")
			parts := splitBindKey(key)
			if len(parts) == 2 {
				if m.Bind[parts[0]] == nil {
					m.Bind[parts[0]] = make(map[string]string)
				}
				m.Bind[parts[0]][parts[1]] = module
			}
		}
	}

	// Map serve to DomainServe.
	if bm.Serve != nil {
		m.DomainServe = &DomainServeConfig{
			Port:   bm.Serve.Port,
			Assets: &AssetMap{},
		}
	}

	// Map prompts to assets.
	if m.DomainServe != nil && m.DomainServe.Assets != nil && len(bm.Prompts) > 0 {
		m.DomainServe.Assets.Prompts = bm.Prompts
	}

	// Map params.
	m.Params = bm.Params

	return m
}

// splitBindKey splits "rca.source" into ["rca", "source"].
func splitBindKey(key string) []string {
	idx := strings.Index(key, ".")
	if idx < 0 {
		return []string{key}
	}
	return []string{key[:idx], key[idx+1:]}
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
	if err := validateCircuitRefs(m, manifestDir); err != nil {
		return err
	}
	if err := validatePortWiring(m, manifestDir); err != nil {
		return err
	}
	return ValidateDomainKinds(m, manifestDir)
}

func validateNoDuplicateDomains(m *Manifest) error {
	seen := make(map[string]bool)
	for _, d := range m.Domains {
		if seen[d] {
			return fmt.Errorf("%w: %q", ErrManifestDuplicateDomain, d)
		}
		seen[d] = true
	}
	return nil
}

func validateDomainDirs(m *Manifest, manifestDir string, verbose bool) error {
	for _, d := range m.Domains {
		dir := filepath.Join(manifestDir, "domains", d)
		if info, err := os.Stat(dir); err != nil || !info.IsDir() {
			return fmt.Errorf("%w: %q declared in manifest but domains/%s/ not found", ErrDomain, d, d)
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
			return fmt.Errorf("%w: %q not found on disk", ErrAssetPath, p)
		}
	}
	return nil
}

const (
	origamiModule = "github.com/dpopsuev/origami"
	mcpSDKModule  = "github.com/modelcontextprotocol/go-sdk"
)

func buildWiredBinary(ctx context.Context, m *Manifest, opts *Options) error {
	resolver := opts.ModuleResolver
	if resolver == nil {
		resolver = &DefaultModuleResolver{}
	}

	origamiRoot := resolver.FindLocalModule(origamiModule)
	if origamiRoot == "" {
		return ErrCannotFindOrigamiModuleOnLocalFilesystem
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

	if err := os.WriteFile(filepath.Join(tmpDir, "main.go"), src, 0o600); err != nil {
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

	if err := createWiredBuildModule(tmpDir, m.Name, resolver, g, opts.Local); err != nil {
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

	if err := os.MkdirAll(filepath.Dir(output), 0o755); err != nil {
		return fmt.Errorf("create output dir: %w", err)
	}

	ldflags := buildLDFlags(opts.ManifestPath)
	args := []string{"build", "-ldflags", ldflags, "-o", output}
	args = append(args, opts.GoFlags...)
	args = append(args, ".")

	cmd := exec.CommandContext(ctx, "go", args...)
	cmd.Dir = tmpDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = append(os.Environ(), "CGO_ENABLED=0")

	if opts.Verbose {
		fmt.Fprintf(os.Stderr, "running: CGO_ENABLED=0 go %s (in %s)\n", strings.Join(args, " "), tmpDir)
	}

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("go build wired binary: %w", err)
	}

	fmt.Fprintf(os.Stderr, "built %s\n", output)
	return nil
}

func createWiredBuildModule(tmpDir, name string, resolver ModuleResolver, g *ResolvedGraph, local bool) error {
	var buf strings.Builder
	buf.WriteString(fmt.Sprintf("module %s-build\n\ngo 1.24\n\nrequire (\n", name))

	origamiVersion := resolveModuleVersion(resolver, origamiModule)
	mcpVersion := resolveModuleVersion(resolver, mcpSDKModule)
	buf.WriteString(fmt.Sprintf("\t%s %s\n", origamiModule, origamiVersion))
	buf.WriteString(fmt.Sprintf("\t%s %s\n", mcpSDKModule, mcpVersion))

	// Collect unique external module roots from resolved imports.
	seen := map[string]bool{origamiModule: true, mcpSDKModule: true}
	var externalModules []string
	if g != nil {
		for _, imp := range g.Imports {
			mod := moduleRoot(imp.Path)
			if mod != "" && !seen[mod] {
				seen[mod] = true
				externalModules = append(externalModules, mod)
				v := resolveModuleVersion(resolver, mod)
				buf.WriteString(fmt.Sprintf("\t%s %s\n", mod, v))
			}
		}
	}
	buf.WriteString(")\n\n")

	// Add replace directives only when explicitly requested (--local flag).
	if local {
		if localPath := resolver.FindLocalModule(origamiModule); localPath != "" {
			fmt.Fprintf(os.Stderr, "WARNING: using local module %s => %s\n", origamiModule, localPath)
			buf.WriteString(fmt.Sprintf("replace %s => %s\n", origamiModule, localPath))
		}
		for _, mod := range externalModules {
			if localPath := resolver.FindLocalModule(mod); localPath != "" {
				fmt.Fprintf(os.Stderr, "WARNING: using local module %s => %s\n", mod, localPath)
				buf.WriteString(fmt.Sprintf("replace %s => %s\n", mod, localPath))
			}
		}
	}

	return os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte(buf.String()), 0o600)
}

const fallbackVersion = "v0.0.0"

// resolveModuleVersion reads the version of a dependency from origami's own go.mod.
// Falls back to fallbackVersion if not found (e.g., when origami IS the module).
func resolveModuleVersion(resolver ModuleResolver, modPath string) string {
	origamiRoot := resolver.FindLocalModule(origamiModule)
	if origamiRoot == "" {
		return fallbackVersion
	}
	goModPath := filepath.Join(origamiRoot, "go.mod")
	data, err := os.ReadFile(goModPath)
	if err != nil {
		return fallbackVersion
	}
	// Parse "require modPath vX.Y.Z" from go.mod
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, modPath+" ") {
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				version := parts[1]
				// Strip // indirect suffix
				if i := strings.Index(version, " "); i > 0 {
					version = version[:i]
				}
				return version
			}
		}
	}
	// If the module wasn't found in origami's go.mod, try reading the
	// latest git tag from the local module checkout.
	localPath := resolver.FindLocalModule(modPath)
	if localPath != "" {
		if v := readGitTag(localPath); v != "" {
			return v
		}
	}
	return fallbackVersion
}

// readGitTag reads the latest semver tag from a git repository.
func readGitTag(repoDir string) string {
	cmd := exec.Command("git", "describe", "--tags", "--abbrev=0", "--match", "v*")
	cmd.Dir = repoDir
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// moduleRoot extracts the module root from a Go import path.
// "github.com/dpopsuev/origami-rca/connectors/rp" → "github.com/dpopsuev/origami-rca"
// Returns "" for standard library or origami-internal paths.
func moduleRoot(importPath string) string {
	parts := strings.Split(importPath, "/")
	if len(parts) < 3 || !strings.Contains(parts[0], ".") {
		return ""
	}
	return strings.Join(parts[:3], "/")
}

func buildDomainServe(ctx context.Context, m *Manifest, opts *Options) error {
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

	if err := os.WriteFile(filepath.Join(tmpDir, "main.go"), src, 0o600); err != nil {
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

	if err := createDomainServeBuildModule(tmpDir, m.Name, resolver, opts.Local); err != nil {
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

	if err := os.MkdirAll(filepath.Dir(output), 0o755); err != nil {
		return fmt.Errorf("create output dir: %w", err)
	}

	ldflags := buildLDFlags(opts.ManifestPath)
	args := []string{"build", "-ldflags", ldflags, "-o", output}
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

func buildContainerImage(ctx context.Context, m *Manifest, binaryPath string, opts *Options) error {
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
	if err := os.WriteFile(filepath.Join(imgDir, "Dockerfile"), []byte(dockerfile), 0o600); err != nil {
		return fmt.Errorf("write Dockerfile: %w", err)
	}

	src, err := os.Open(binaryPath)
	if err != nil {
		return fmt.Errorf("open binary: %w", err)
	}
	defer src.Close()

	dst, err := os.OpenFile(filepath.Join(imgDir, "domain-serve"), os.O_CREATE|os.O_WRONLY, 0o755)
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

func createDomainServeBuildModule(tmpDir, name string, resolver ModuleResolver, local bool) error {
	var buf strings.Builder
	buf.WriteString(fmt.Sprintf("module %s-domain-serve-build\n\ngo 1.24\n\nrequire (\n", name))
	buf.WriteString(fmt.Sprintf("\t%s v0.0.0\n", origamiModule))
	buf.WriteString(")\n\n")

	if local {
		if localPath := resolver.FindLocalModule(origamiModule); localPath != "" {
			fmt.Fprintf(os.Stderr, "WARNING: using local module %s => %s\n", origamiModule, localPath)
			buf.WriteString(fmt.Sprintf("replace %s => %s\n", origamiModule, localPath))
		}
	}

	return os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte(buf.String()), 0o600)
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

// exportDataDir copies flattened domain data to a directory for use with
// the --data-dir runtime flag. This produces the same file layout that
// the go:embed directive would create, making it suitable for volume mounts.
// The target directory is cleaned before each export to prevent stale files.
func exportDataDir(m *Manifest, manifestDir string, opts *Options) error {
	if err := m.MergeDiscoveredAssets(manifestDir); err != nil {
		return fmt.Errorf("discover domain assets: %w", err)
	}

	// Clean target to prevent stale files from prior exports.
	if err := os.RemoveAll(opts.ExportDataDir); err != nil {
		return fmt.Errorf("clean export dir: %w", err)
	}
	if err := os.MkdirAll(opts.ExportDataDir, 0o755); err != nil {
		return fmt.Errorf("create export dir: %w", err)
	}

	if err := copyDomainFiles(m, manifestDir, opts.ExportDataDir, opts.Verbose); err != nil {
		return err
	}
	if err := copyEmbedFiles(m.DomainServe, manifestDir, opts.ExportDataDir, opts.Verbose); err != nil {
		return err
	}

	fmt.Fprintf(os.Stderr, "exported domain data to %s\n", opts.ExportDataDir)
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
				return fmt.Errorf("%w: %q references circuit %q which is not in assets.circuits", ErrCircuit, name, ref)
			}
		}
		deps[name] = refs
	}

	if cycle := detectCircuitCycle(deps); cycle != "" {
		return fmt.Errorf("%w: %s", ErrCircuitDependencyCycleDetected, cycle)
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

// circuitPortsForValidation is a minimal struct for extracting port and wiring
// declarations from circuit YAML files during fold validation.
type circuitPortsForValidation struct {
	Circuit string `yaml:"circuit"`
	Ports   []struct {
		Name string `yaml:"name"`
		Type string `yaml:"type"`
	} `yaml:"ports"`
	Wiring []struct {
		From string `yaml:"from"`
		To   string `yaml:"to"`
	} `yaml:"wiring"`
}

// validatePortWiring checks that wiring entries across circuits connect ports
// with matching type declarations. A type mismatch (e.g. TriageResult vs
// []string) is reported as an error at fold time rather than at runtime.
//
//nolint:gocyclo // cross-circuit port type validation — inherently branchy
func validatePortWiring(m *Manifest, manifestDir string) error {
	if m.DomainServe == nil || m.DomainServe.Assets == nil {
		return nil
	}
	circuits := m.DomainServe.Assets.Circuits
	if len(circuits) == 0 {
		return nil
	}

	// Load all circuit files and collect port types + wiring entries.
	// circuitName → portName → type
	portIndex := make(map[string]map[string]string)

	type wiringEntry struct {
		from        string
		to          string
		circuitFile string
	}
	var allWiring []wiringEntry

	for name, path := range circuits {
		data, err := os.ReadFile(filepath.Join(manifestDir, path))
		if err != nil {
			continue // file-not-found is handled by validateAssetPaths
		}
		var cf circuitPortsForValidation
		if err := yaml.Unmarshal(data, &cf); err != nil {
			continue // parse errors are reported elsewhere
		}

		circuitName := cf.Circuit
		if circuitName == "" {
			circuitName = name
		}

		if len(cf.Ports) > 0 {
			ports := make(map[string]string, len(cf.Ports))
			for _, p := range cf.Ports {
				ports[p.Name] = p.Type
			}
			portIndex[circuitName] = ports
		}

		for _, w := range cf.Wiring {
			allWiring = append(allWiring, wiringEntry{
				from:        w.From,
				to:          w.To,
				circuitFile: path,
			})
		}
	}

	if len(allWiring) == 0 {
		return nil
	}

	// Check each wiring entry for port type compatibility.
	for _, w := range allWiring {
		fromCircuit, fromPort := parseWiringRef(w.from)
		toCircuit, toPort := parseWiringRef(w.to)

		if fromCircuit == "" || fromPort == "" || toCircuit == "" || toPort == "" {
			continue // malformed — skip
		}

		fromPorts, fromOK := portIndex[fromCircuit]
		toPorts, toOK := portIndex[toCircuit]
		if !fromOK || !toOK {
			continue // circuit not in manifest — can't check
		}

		fromType, fromExists := fromPorts[fromPort]
		toType, toExists := toPorts[toPort]
		if !fromExists || !toExists {
			continue // port not declared — can't check
		}

		if fromType == "" || toType == "" {
			continue // no type declared — nothing to compare
		}

		if fromType != toType {
			return fmt.Errorf("%w: %s → %s: type mismatch: %s has type %q but %s has type %q", ErrPortWiring, w.from, w.to, w.from, fromType, w.to, toType)
		}
	}

	return nil
}

// parseWiringRef parses a wiring reference like "rca.out:post-triage"
// into (circuit, port_name).
func parseWiringRef(ref string) (circuitName, port string) {
	dotIdx := strings.Index(ref, ".")
	if dotIdx < 0 {
		return "", ""
	}
	circuitName = ref[:dotIdx]
	rest := ref[dotIdx+1:]
	_, port, _ = strings.Cut(rest, ":")
	return circuitName, port
}

func copyFile(src, dst string) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
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
