package lint

// AllRules returns all built-in lint rules ordered by category:
// structural (S), semantic (G), best-practice (B), prompt (P).
func AllRules() []Rule {
	all := append(structuralRules(), semanticRules()...)
	all = append(all, bestPracticeRules()...)
	all = append(all, promptRules()...)
	all = append(all, crossRefRules()...)
	all = append(all, scenarioRules()...)
	return all
}

func structuralRules() []Rule {
	return []Rule{
		&MissingEdgeID{},
		&MissingNodeApproach{},
		&InvalidApproach{},
		&InvalidMergeStrategy{},
		&MissingEdgeName{},
		&DuplicateEdgeCondition{},
		&EmptyPrompt{},
		&InvalidCacheTTL{},
		&MissingCircuitDescription{},
		&UnnamedNode{},
		&InvalidWalkerApproach{},
		&InvalidWalkerPersona{},
		&SchemaInUnstructuredZone{},
		&MissingZoneDomain{},
		&InvalidZoneDomain{},
		&DelegateWithoutGenerator{},
		&InvalidWalkerRole{},
		&DeprecatedHandlerFields{},
		&ImportOverlay{},
		&PortValidation{},
		&CalibrationContract{},
		&EdgeNodeReference{},
		&HookReference{},
		&InvalidHandlerType{},
	}
}

func semanticRules() []Rule {
	return []Rule{
		&OrphanNode{},
		&UnreachableDone{},
		&DeadEdge{},
		&ShortcutBypassesRequired{},
		&ZoneApproachMismatch{},
		&ExpressionCompileError{},
		&FanInWithoutMerge{},
		&UnacknowledgedShortcut{},
		&UnacknowledgedLoop{},
	}
}

func bestPracticeRules() []Rule {
	return []Rule{
		&PreferWhenOverCondition{},
		&NameYourEdges{},
		&TerminalEdgeToDone{},
		&ZoneStickinessWithoutProvider{},
		&LargeCircuitNoZones{},
		&ApproachAffinityChain{},
		&StochasticTransformer{},
		&StochasticSummary{},
		&MissingKind{},
		&DeprecatedArrow{},
		&MissingKindDomainPath{},
	}
}

func promptRules() []Rule {
	return []Rule{
		&TemplateParamValidity{},
	}
}

func crossRefRules() []Rule {
	return []Rule{
		&CrossRefEngine{Rules: DefaultCrossRefRules()},
	}
}

func scenarioRules() []Rule {
	return []Rule{
		&ExpectedPathNodeNames{},
		&CircuitHandlerResolution{},
		&DeadNodeDetection{},
		&MediatorBackendCoverage{},
		&PortTypeConsistency{},
	}
}
