package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/dpopsuev/tako/calibrate"
)

func validateBundleCmd(args []string) error {
	fs := flag.NewFlagSet("validate-bundle", flag.ContinueOnError)
	path := fs.String("path", "", "path to bundle directory")
	checkSHA := fs.Bool("check-sha", false, "verify file checksums against manifest")
	if err := fs.Parse(args); err != nil {
		return err
	}

	if *path == "" {
		return ErrUsageTakoValidateBundlePathDirCheckSha
	}

	bundleFS := os.DirFS(*path)

	errs := calibrate.ValidateBundle(bundleFS, *checkSHA)
	if len(errs) == 0 {
		fmt.Printf("OK: bundle at %s is valid\n", *path)
		return nil
	}

	for _, err := range errs {
		fmt.Fprintf(os.Stderr, "  ERROR: %v\n", err)
	}
	return fmt.Errorf("%w: %d error(s)", ErrBundleValidationFailedWith, len(errs))
}
