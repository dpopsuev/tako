package framework

// Category: Execution — aliases to engine/ package.

import "github.com/dpopsuev/origami/engine"

const ArtifactStoreKey = engine.ArtifactStoreKey

type ArtifactStore = engine.ArtifactStore
type ParallelEnforcerConfig = engine.ParallelEnforcerConfig

var RunWithEnforcer = engine.RunWithEnforcer
