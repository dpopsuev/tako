package main

import (
	"context"
	"flag"

	"github.com/dpopsuev/tako/fold"
)

func foldCmd(args []string) error {
	fs := flag.NewFlagSet("fold", flag.ContinueOnError)
	output := fs.String("output", "", "output binary path (default: bin/<name>-domain-serve)")
	container := fs.Bool("container", false, "build an OCI container image after compiling")
	domainOnly := fs.Bool("domain-only", false, "build domain-serve binary only (ignore schematics/connectors)")
	imageName := fs.String("image", "", "container image name (default: tako-<name>-domain)")
	exportData := fs.String("export-data", "", "export flattened domain data to this directory (for volume mounts)")
	local := fs.Bool("local", false, "use local module overrides via replace directives (dev only)")
	verbose := fs.Bool("v", false, "verbose output")
	if err := fs.Parse(args); err != nil {
		return err
	}

	manifest := "tako.yaml"
	if fs.NArg() > 0 {
		manifest = fs.Arg(0)
	}

	return fold.Run(context.Background(), &fold.Options{
		ManifestPath:  manifest,
		Output:        *output,
		Container:     *container,
		DomainOnly:    *domainOnly,
		ImageName:     *imageName,
		ExportDataDir: *exportData,
		Local:         *local,
		Verbose:       *verbose,
	})
}
