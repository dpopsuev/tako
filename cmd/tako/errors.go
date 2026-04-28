package main

import "errors"

var (
	// ErrNoCircuitsFoundIn is returned for: no circuits found in
	ErrNoCircuitsFoundIn = errors.New("no circuits found in")

	// ErrCircuitStart is returned for: circuit/start
	ErrCircuitStart = errors.New("circuit/start")

	// ErrCircuitReport is returned for: circuit/report
	ErrCircuitReport = errors.New("circuit/report")

	// ErrUsageTakoCaptureSchematicNameSourcePackPathOutput is returned for: usage: tako capture --schematic=<name> --source-pack=<path> --output=<dir>
	ErrUsageTakoCaptureSchematicNameSourcePackPathOutput = errors.New("usage: tako capture --schematic=<name> --source-pack=<path> --output=<dir>")

	// ErrUsageTakoComponentListInspectValidateFlags is returned for: usage: tako component <list|inspect|validate> [flags]
	ErrUsageTakoComponentListInspectValidateFlags = errors.New("usage: tako component <list|inspect|validate> [flags]")

	// ErrUnknownComponentSubcommand is returned for: unknown component subcommand
	ErrUnknownComponentSubcommand = errors.New("unknown component subcommand")

	// ErrUsageTakoComponentInspectComponentYaml is returned for: usage: tako component inspect <component.yaml>
	ErrUsageTakoComponentInspectComponentYaml = errors.New("usage: tako component inspect <component.yaml>")

	// ErrUsageTakoComponentValidateComponentYaml is returned for: usage: tako component validate <component.yaml>
	ErrUsageTakoComponentValidateComponentYaml = errors.New("usage: tako component validate <component.yaml>")

	// ErrComponentManifest is returned for: component manifest
	ErrComponentManifest = errors.New("component manifest")

	// ErrUsageTakoDiffStateDirDIRRunARunBFormatTextJson is returned for: usage: tako diff [--state-dir=DIR] <run-a> <run-b> [--format=text|json]
	ErrUsageTakoDiffStateDirDIRRunARunBFormatTextJson = errors.New("usage: tako diff [--state-dir=DIR] <run-a> <run-b> [--format=text|json]")

	// ErrUnknownFormat is returned for: unknown format
	ErrUnknownFormat = errors.New("unknown format")

	// ErrRunDirectoryNotFound is returned for: run directory not found
	ErrRunDirectoryNotFound = errors.New("run directory not found")

	// ErrNoRunsFoundIn is returned for: no runs found in
	ErrNoRunsFoundIn = errors.New("no runs found in")

	// ErrUsageTakoSkillScaffoldFlags is returned for: usage: tako skill <scaffold> [flags]
	ErrUsageTakoSkillScaffoldFlags = errors.New("usage: tako skill <scaffold> [flags]")

	// ErrUnknownSkillSubcommand is returned for: unknown skill subcommand
	ErrUnknownSkillSubcommand = errors.New("unknown skill subcommand")

	// ErrUsageTakoSkillScaffoldToolNAMEOutDIRCircuitYaml is returned for: usage: tako skill scaffold [--tool NAME] [--out DIR] <circuit.yaml>
	ErrUsageTakoSkillScaffoldToolNAMEOutDIRCircuitYaml = errors.New("usage: tako skill scaffold [--tool NAME] [--out DIR] <circuit.yaml>")

	// ErrUsageTakoValidateBundlePathDirCheckSha is returned for: usage: tako validate-bundle --path=<dir> [--check-sha]
	ErrUsageTakoValidateBundlePathDirCheckSha = errors.New("usage: tako validate-bundle --path=<dir> [--check-sha]")

	// ErrBundleValidationFailedWith is returned for: bundle validation failed with
	ErrBundleValidationFailedWith = errors.New("bundle validation failed with")

	// ErrExpectedKeyValueGot is returned for: expected key=value, got
	ErrExpectedKeyValueGot = errors.New("expected key=value, got")

	// ErrUsageTakoRunVSetKeyValueCircuitYaml is returned for: usage: tako run [-v] [--set key=value] <circuit.yaml>
	ErrUsageTakoRunVSetKeyValueCircuitYaml = errors.New("usage: tako run [-v] [--set key=value] <circuit.yaml>")

	// ErrUsageTakoValidateCircuitYaml is returned for: usage: tako validate <circuit.yaml>
	ErrUsageTakoValidateCircuitYaml = errors.New("usage: tako validate <circuit.yaml>")

	// ErrUsageTakoLintProfileNameFormatTextJsonFixFileYaml is returned for: usage: tako lint [--profile <name>] [--format text|json] [--fix] <file.yaml>...
	ErrUsageTakoLintProfileNameFormatTextJsonFixFileYaml = errors.New("usage: tako lint [--profile <name>] [--format text|json] [--fix] <file.yaml>...")

	// errYAMLPath is returned when a YAML path cannot be resolved.
	errYAMLPath = errors.New("yaml path")
)
