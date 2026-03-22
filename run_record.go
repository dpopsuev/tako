package framework

// Category: Execution — aliases to engine/ package.

import "github.com/dpopsuev/origami/engine"

type RunRecord = engine.RunRecord

var (
	SaveRunRecord = engine.SaveRunRecord
	LoadRunRecord = engine.LoadRunRecord
)
