package fold

import "errors"

var (
	// ErrManifestHasNoSchematicsOrUsesSection is returned for: manifest has no schematics or uses section
	ErrManifestHasNoSchematicsOrUsesSection = errors.New("manifest has no schematics or uses section")

	// ErrSchematic is returned for: schematic
	ErrSchematic = errors.New("schematic")

	// ErrNoRootSchematicFoundAllSchematicsAreDependenciesOfOt is returned for: no root schematic found (all schematics are dependencies of others)
	ErrNoRootSchematicFoundAllSchematicsAreDependenciesOfOt = errors.New("no root schematic found (all schematics are dependencies of others)")

	// ErrMultipleRootSchematics is returned for: multiple root schematics
	ErrMultipleRootSchematics = errors.New("multiple root schematics")

	// ErrCycleDetected is returned for: cycle detected
	ErrCycleDetected = errors.New("cycle detected")

	// ErrManifestHasNoDomainServeSection is returned for: manifest has no domain_serve section
	ErrManifestHasNoDomainServeSection = errors.New("manifest has no domain_serve section")

	// ErrDomainServeAssetsIsRequired is returned for: domain_serve: assets is required
	ErrDomainServeAssetsIsRequired = errors.New("domain_serve: assets is required")

	// ErrManifestMustHaveADomainServeSection is returned for: manifest must have a domain_serve section
	ErrManifestMustHaveADomainServeSection = errors.New("manifest must have a domain_serve section")

	// ErrManifestDuplicateDomain is returned for: manifest: duplicate domain
	ErrManifestDuplicateDomain = errors.New("manifest: duplicate domain")

	// ErrDomain is returned for: domain
	ErrDomain = errors.New("domain")

	// ErrAssetPath is returned for: asset path
	ErrAssetPath = errors.New("asset path")

	// ErrCannotFindOrigamiModuleOnLocalFilesystem is returned for: cannot find origami module on local filesystem
	ErrCannotFindOrigamiModuleOnLocalFilesystem = errors.New("cannot find origami module on local filesystem")

	// ErrCircuit is returned for: circuit
	ErrCircuit = errors.New("circuit")

	// ErrCircuitDependencyCycleDetected is returned for: circuit dependency cycle detected
	ErrCircuitDependencyCycleDetected = errors.New("circuit dependency cycle detected")

	// ErrPortWiring is returned for: port wiring
	ErrPortWiring = errors.New("port wiring")

	// ErrManifestApiVersionMustBeOrigamiV1Got is returned for: manifest: apiVersion must be 'origami/v1', got
	ErrManifestApiVersionMustBeOrigamiV1Got = errors.New("manifest: apiVersion must be 'origami/v1', got")

	// ErrManifestKindMustBeBoardGot is returned for: manifest: kind must be 'Board', got
	ErrManifestKindMustBeBoardGot = errors.New("manifest: kind must be 'Board', got")

	// ErrManifestMetadataNameIsRequired is returned for: manifest: metadata.name is required
	ErrManifestMetadataNameIsRequired = errors.New("manifest: metadata.name is required")

	// ErrUses is returned for: uses
	ErrUses = errors.New("uses")

	// ErrBind is returned for: bind
	ErrBind = errors.New("bind")

	// ErrDomainKindMismatch is returned when a domain file's kind header doesn't match expected.
	ErrDomainKindMismatch = errors.New("domain kind mismatch")

	// ErrBoardKindMismatch is returned when a board file has wrong kind.
	ErrBoardKindMismatch = errors.New("board: kind mismatch")

	// ErrBoardNameRequired is returned when a board file has no name.
	ErrBoardNameRequired = errors.New("board: name is required")

	// ErrCompositionCycle is returned when board composition has a cycle.
	ErrCompositionCycle = errors.New("board composition: cycle detected")

	// ErrInstrument is returned for instrument manifest errors during fold.
	ErrInstrument = errors.New("instrument")
)
