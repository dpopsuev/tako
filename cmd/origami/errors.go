package main

import "errors"

var (
	// ErrNoCircuitsFoundIn is returned for: no circuits found in
	ErrNoCircuitsFoundIn = errors.New("no circuits found in")

	// ErrCircuitStart is returned for: circuit/start
	ErrCircuitStart = errors.New("circuit/start")

	// ErrCircuitReport is returned for: circuit/report
	ErrCircuitReport = errors.New("circuit/report")

	// ErrUsageOrigamiCaptureSchematicNameSourcePackPathOutput is returned for: usage: origami capture --schematic=<name> --source-pack=<path> --output=<dir>
	ErrUsageOrigamiCaptureSchematicNameSourcePackPathOutput = errors.New("usage: origami capture --schematic=<name> --source-pack=<path> --output=<dir>")

	// ErrUsageOrigamiComponentListInspectValidateFlags is returned for: usage: origami component <list|inspect|validate> [flags]
	ErrUsageOrigamiComponentListInspectValidateFlags = errors.New("usage: origami component <list|inspect|validate> [flags]")

	// ErrUnknownComponentSubcommand is returned for: unknown component subcommand
	ErrUnknownComponentSubcommand = errors.New("unknown component subcommand")

	// ErrUsageOrigamiComponentInspectComponentYaml is returned for: usage: origami component inspect <component.yaml>
	ErrUsageOrigamiComponentInspectComponentYaml = errors.New("usage: origami component inspect <component.yaml>")

	// ErrUsageOrigamiComponentValidateComponentYaml is returned for: usage: origami component validate <component.yaml>
	ErrUsageOrigamiComponentValidateComponentYaml = errors.New("usage: origami component validate <component.yaml>")

	// ErrComponentManifest is returned for: component manifest
	ErrComponentManifest = errors.New("component manifest")

	// ErrUsageOrigamiDiffStateDirDIRRunARunBFormatTextJson is returned for: usage: origami diff [--state-dir=DIR] <run-a> <run-b> [--format=text|json]
	ErrUsageOrigamiDiffStateDirDIRRunARunBFormatTextJson = errors.New("usage: origami diff [--state-dir=DIR] <run-a> <run-b> [--format=text|json]")

	// ErrUnknownFormat is returned for: unknown format
	ErrUnknownFormat = errors.New("unknown format")

	// ErrRunDirectoryNotFound is returned for: run directory not found
	ErrRunDirectoryNotFound = errors.New("run directory not found")

	// ErrNoRunsFoundIn is returned for: no runs found in
	ErrNoRunsFoundIn = errors.New("no runs found in")

	// ErrUsageOrigamiSkillScaffoldFlags is returned for: usage: origami skill <scaffold> [flags]
	ErrUsageOrigamiSkillScaffoldFlags = errors.New("usage: origami skill <scaffold> [flags]")

	// ErrUnknownSkillSubcommand is returned for: unknown skill subcommand
	ErrUnknownSkillSubcommand = errors.New("unknown skill subcommand")

	// ErrUsageOrigamiSkillScaffoldToolNAMEOutDIRCircuitYaml is returned for: usage: origami skill scaffold [--tool NAME] [--out DIR] <circuit.yaml>
	ErrUsageOrigamiSkillScaffoldToolNAMEOutDIRCircuitYaml = errors.New("usage: origami skill scaffold [--tool NAME] [--out DIR] <circuit.yaml>")

	// ErrUsageOrigamiValidateBundlePathDirCheckSha is returned for: usage: origami validate-bundle --path=<dir> [--check-sha]
	ErrUsageOrigamiValidateBundlePathDirCheckSha = errors.New("usage: origami validate-bundle --path=<dir> [--check-sha]")

	// ErrBundleValidationFailedWith is returned for: bundle validation failed with
	ErrBundleValidationFailedWith = errors.New("bundle validation failed with")

	// ErrExpectedKeyValueGot is returned for: expected key=value, got
	ErrExpectedKeyValueGot = errors.New("expected key=value, got")

	// ErrUsageOrigamiRunVSetKeyValueCircuitYaml is returned for: usage: origami run [-v] [--set key=value] <circuit.yaml>
	ErrUsageOrigamiRunVSetKeyValueCircuitYaml = errors.New("usage: origami run [-v] [--set key=value] <circuit.yaml>")

	// ErrUsageOrigamiValidateCircuitYaml is returned for: usage: origami validate <circuit.yaml>
	ErrUsageOrigamiValidateCircuitYaml = errors.New("usage: origami validate <circuit.yaml>")

	// ErrUsageOrigamiLintProfileNameFormatTextJsonFixFileYaml is returned for: usage: origami lint [--profile <name>] [--format text|json] [--fix] <file.yaml>...
	ErrUsageOrigamiLintProfileNameFormatTextJsonFixFileYaml = errors.New("usage: origami lint [--profile <name>] [--format text|json] [--fix] <file.yaml>...")

	// errYAMLPath is returned when a YAML path cannot be resolved.
	errYAMLPath = errors.New("yaml path")
)
