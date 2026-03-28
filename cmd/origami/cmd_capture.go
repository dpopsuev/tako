package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"

	"github.com/dpopsuev/origami/calibrate"
)

func captureCmd(args []string) error {
	fs := flag.NewFlagSet("capture", flag.ContinueOnError)
	schematic := fs.String("schematic", "", "schematic name (e.g. gnd)")
	sourcePack := fs.String("source-pack", "", "path to source pack YAML")
	output := fs.String("output", "", "output directory for the bundle")
	overwrite := fs.Bool("overwrite", false, "overwrite existing bundle")
	verbose := fs.Bool("v", false, "verbose output")
	if err := fs.Parse(args); err != nil {
		return err
	}

	if *schematic == "" || *sourcePack == "" || *output == "" {
		return fmt.Errorf("usage: origami capture --schematic=<name> --source-pack=<path> --output=<dir>")
	}

	level := slog.LevelInfo
	if *verbose {
		level = slog.LevelDebug
	}
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level}))
	_ = logger // available for capturers that need it

	capturer, err := calibrate.GetCapturer(*schematic)
	if err != nil {
		return fmt.Errorf("schematic %q: %w (capturers are registered by schematics at init time; use a folded binary)", *schematic, err)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	cfg := calibrate.CaptureConfig{
		Schematic:  *schematic,
		SourcePack: *sourcePack,
		OutputDir:  *output,
		Overwrite:  *overwrite,
	}

	if err := capturer.Capture(ctx, cfg); err != nil {
		return fmt.Errorf("capture failed: %w", err)
	}

	fmt.Fprintf(os.Stderr, "capture complete: %s\n", *output)
	return nil
}
