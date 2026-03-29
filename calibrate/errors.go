package calibrate

import "errors"

var (
	// ErrBatchScorerExpectedMapStringAnyGot is returned for: batch scorer: expected []map[string]any, got
	ErrBatchScorerExpectedMapStringAnyGot = errors.New("batch scorer: expected []map[string]any, got")

	// ErrBatchFieldMatchRequiresActualAndExpectedParams is returned for: batch_field_match: requires 'actual' and 'expected' params
	ErrBatchFieldMatchRequiresActualAndExpectedParams = errors.New("batch_field_match: requires 'actual' and 'expected' params")

	// ErrBatchBoolRateRequiresFilterFieldAndActualFieldParams is returned for: batch_bool_rate: requires 'filter_field' and 'actual_field' params
	ErrBatchBoolRateRequiresFilterFieldAndActualFieldParams = errors.New("batch_bool_rate: requires 'filter_field' and 'actual_field' params")

	// ErrBatchSetPrecisionRequiresActualFieldAndRelevantField is returned for: batch_set_precision: requires 'actual_field' and 'relevant_field' params
	ErrBatchSetPrecisionRequiresActualFieldAndRelevantField = errors.New("batch_set_precision: requires 'actual_field' and 'relevant_field' params")

	// ErrBatchSetRecallRequiresActualFieldAndRelevantFieldPar is returned for: batch_set_recall: requires 'actual_field' and 'relevant_field' params
	ErrBatchSetRecallRequiresActualFieldAndRelevantFieldPar = errors.New("batch_set_recall: requires 'actual_field' and 'relevant_field' params")

	// ErrBatchSetExclusionRequiresActualFieldAndExcludedField is returned for: batch_set_exclusion: requires 'actual_field' and 'excluded_field' params
	ErrBatchSetExclusionRequiresActualFieldAndExcludedField = errors.New("batch_set_exclusion: requires 'actual_field' and 'excluded_field' params")

	// ErrBatchKeywordScoreRequiresTextFieldAndKeywordsFieldPa is returned for: batch_keyword_score: requires 'text_field' and 'keywords_field' params
	ErrBatchKeywordScoreRequiresTextFieldAndKeywordsFieldPa = errors.New("batch_keyword_score: requires 'text_field' and 'keywords_field' params")

	// ErrBatchCorrelationRequiresXFieldAndYFieldParams is returned for: batch_correlation: requires 'x_field' and 'y_field' params
	ErrBatchCorrelationRequiresXFieldAndYFieldParams = errors.New("batch_correlation: requires 'x_field' and 'y_field' params")

	// ErrBatchSumRatioRequiresNumeratorFieldAndDenominatorFie is returned for: batch_sum_ratio: requires 'numerator_field' and 'denominator_field' params
	ErrBatchSumRatioRequiresNumeratorFieldAndDenominatorFie = errors.New("batch_sum_ratio: requires 'numerator_field' and 'denominator_field' params")

	// ErrBatchFieldSumRequiresFieldParam is returned for: batch_field_sum: requires 'field' param
	ErrBatchFieldSumRequiresFieldParam = errors.New("batch_field_sum: requires 'field' param")

	// ErrBatchGroupLinkageRequiresGroupFieldAndValueFieldPara is returned for: batch_group_linkage: requires 'group_field' and 'value_field' params
	ErrBatchGroupLinkageRequiresGroupFieldAndValueFieldPara = errors.New("batch_group_linkage: requires 'group_field' and 'value_field' params")

	// ErrNoCapturerRegisteredForSchematic is returned for: no capturer registered for schematic
	ErrNoCapturerRegisteredForSchematic = errors.New("no capturer registered for schematic")

	// ErrNoValidatorRegisteredForSchematic is returned for: no validator registered for schematic
	ErrNoValidatorRegisteredForSchematic = errors.New("no validator registered for schematic")

	// ErrCircuitDidNotProduceAReportArtifact is returned for: circuit did not produce a report artifact
	ErrCircuitDidNotProduceAReportArtifact = errors.New("circuit did not produce a report artifact")

	// ErrReportArtifactType is returned for: report artifact type
	ErrReportArtifactType = errors.New("report artifact type")

	// ErrChecksumMismatch is returned for: checksum mismatch
	ErrChecksumMismatch = errors.New("checksum mismatch")

	// ErrUnknownResolution is returned for: unknown resolution
	ErrUnknownResolution = errors.New("unknown resolution")

	// ErrLoadScenarioNoInputInWalkerContext is returned for: load_scenario: no input in walker context
	ErrLoadScenarioNoInputInWalkerContext = errors.New("load_scenario: no input in walker context")

	// ErrLoadScenarioInputType is returned for: load_scenario: input type
	ErrLoadScenarioInputType = errors.New("load_scenario: input type")

	// ErrLoadScenarioScoreCardIsNil is returned for: load_scenario: ScoreCard is nil
	ErrLoadScenarioScoreCardIsNil = errors.New("load_scenario: ScoreCard is nil")

	// ErrLoadScenarioCaseRunnerIsNil is returned for: load_scenario: CaseRunner is nil
	ErrLoadScenarioCaseRunnerIsNil = errors.New("load_scenario: CaseRunner is nil")

	// ErrLoadScenarioCaseScorerIsNil is returned for: load_scenario: CaseScorer is nil
	ErrLoadScenarioCaseScorerIsNil = errors.New("load_scenario: CaseScorer is nil")

	// ErrLoadScenarioNoCases is returned for: load_scenario: no cases
	ErrLoadScenarioNoCases = errors.New("load_scenario: no cases")

	// ErrFanOutNoCalibrationInputInContext is returned for: fan_out: no CalibrationInput in context
	ErrFanOutNoCalibrationInputInContext = errors.New("fan_out: no CalibrationInput in context")

	// ErrWalkCaseNoCalibrationInputInContext is returned for: walk_case: no CalibrationInput in context
	ErrWalkCaseNoCalibrationInputInContext = errors.New("walk_case: no CalibrationInput in context")

	// ErrScoreCaseNoCalibrationInputInContext is returned for: score_case: no CalibrationInput in context
	ErrScoreCaseNoCalibrationInputInContext = errors.New("score_case: no CalibrationInput in context")

	// ErrScoreCaseNoPriorArtifactCaseResults is returned for: score_case: no prior artifact (case_results)
	ErrScoreCaseNoPriorArtifactCaseResults = errors.New("score_case: no prior artifact (case_results)")

	// ErrScoreCasePriorArtifactType is returned for: score_case: prior artifact type
	ErrScoreCasePriorArtifactType = errors.New("score_case: prior artifact type")

	// ErrFanInNoPriorArtifact is returned for: fan_in: no prior artifact
	ErrFanInNoPriorArtifact = errors.New("fan_in: no prior artifact")

	// ErrFanInPriorArtifactType is returned for: fan_in: prior artifact type
	ErrFanInPriorArtifactType = errors.New("fan_in: prior artifact type")

	// ErrAggregateNoCalibrationInputInContext is returned for: aggregate: no CalibrationInput in context
	ErrAggregateNoCalibrationInputInContext = errors.New("aggregate: no CalibrationInput in context")

	// ErrAggregateNoPriorArtifact is returned for: aggregate: no prior artifact
	ErrAggregateNoPriorArtifact = errors.New("aggregate: no prior artifact")

	// ErrAggregatePriorArtifactType is returned for: aggregate: prior artifact type
	ErrAggregatePriorArtifactType = errors.New("aggregate: prior artifact type")

	// ErrReportNoCalibrationInputInContext is returned for: report: no CalibrationInput in context
	ErrReportNoCalibrationInputInContext = errors.New("report: no CalibrationInput in context")

	// ErrReportNoPriorArtifact is returned for: report: no prior artifact
	ErrReportNoPriorArtifact = errors.New("report: no prior artifact")

	// ErrReportPriorArtifactType is returned for: report: prior artifact type
	ErrReportPriorArtifactType = errors.New("report: prior artifact type")

	// ErrPreflightCircuitDefIsRequired is returned for: preflight: CircuitDef is required
	ErrPreflightCircuitDefIsRequired = errors.New("preflight: CircuitDef is required")

	// ErrReportMissingName is returned for: report: missing name
	ErrReportMissingName = errors.New("report: missing name")

	// ErrReportNoSectionsDefined is returned for: report: no sections defined
	ErrReportNoSectionsDefined = errors.New("report: no sections defined")

	// ErrSection is returned for: section
	ErrSection = errors.New("section")

	// ErrRepeatSectionRequiresItemsField is returned for: repeat section requires 'items' field
	ErrRepeatSectionRequiresItemsField = errors.New("repeat section requires 'items' field")

	// ErrData is returned for: data[
	ErrData = errors.New("data[")

	// ErrLoaderIsRequired is returned for: calibrate.Run: Loader is required
	ErrLoaderIsRequired = errors.New("calibrate.Run: Loader is required")

	// ErrCollectorIsRequired is returned for: calibrate.Run: Collector is required
	ErrCollectorIsRequired = errors.New("calibrate.Run: Collector is required")

	// ErrCircuitDefIsRequired is returned for: calibrate.Run: CircuitDef is required
	ErrCircuitDefIsRequired = errors.New("calibrate.Run: CircuitDef is required")

	// ErrScoreCardIsRequired is returned for: calibrate.Run: ScoreCard is required
	ErrScoreCardIsRequired = errors.New("calibrate.Run: ScoreCard is required")

	// ErrCircuitHasHandlerTypeCircuitNode is returned for: circuit has handler_type:circuit node
	ErrCircuitHasHandlerTypeCircuitNode = errors.New("circuit has handler_type:circuit node")

	// ErrCircuitErrorRate is returned for: circuit error rate
	ErrCircuitErrorRate = errors.New("circuit error rate")

	// ErrScenarioHasNoCases is returned for: scenario has no cases
	ErrScenarioHasNoCases = errors.New("scenario has no cases")

	// ErrScenarioNameIsRequired is returned for: scenario name is required
	ErrScenarioNameIsRequired = errors.New("scenario name is required")

	// ErrDuplicateCaseID is returned for: duplicate case ID
	ErrDuplicateCaseID = errors.New("duplicate case ID")

	// ErrCompositeScenarioHasNoCases is returned for: composite scenario has no cases
	ErrCompositeScenarioHasNoCases = errors.New("composite scenario has no cases")

	// ErrUnknownScenario is returned for: unknown scenario
	ErrUnknownScenario = errors.New("unknown scenario")

	// ErrScorecard is returned for: scorecard
	ErrScorecard = errors.New("scorecard")

	// ErrNoAggregateConfigDefined is returned for: no aggregate config defined
	ErrNoAggregateConfigDefined = errors.New("no aggregate config defined")

	// ErrUnsupportedScorecardFormat is returned for: unsupported scorecard format
	ErrUnsupportedScorecardFormat = errors.New("unsupported scorecard format")

	// ErrScorerRegistryIsNil is returned for: scorer registry is nil
	ErrScorerRegistryIsNil = errors.New("scorer registry is nil")

	// ErrScorer is returned for: scorer
	ErrScorer = errors.New("scorer")

	// ErrAccuracyScorerParamsMustIncludePredictedAndExpectedF is returned for: accuracy scorer: params must include 'predicted' and 'expected' field names
	ErrAccuracyScorerParamsMustIncludePredictedAndExpectedF = errors.New("accuracy scorer: params must include 'predicted' and 'expected' field names")

	// ErrAccuracyScorerCaseResultMustBeMapStringAnyGot is returned for: accuracy scorer: caseResult must be map[string]any, got
	ErrAccuracyScorerCaseResultMustBeMapStringAnyGot = errors.New("accuracy scorer: caseResult must be map[string]any, got")

	// ErrAccuracyScorerGroundTruthMustBeMapStringAnyGot is returned for: accuracy scorer: groundTruth must be map[string]any, got
	ErrAccuracyScorerGroundTruthMustBeMapStringAnyGot = errors.New("accuracy scorer: groundTruth must be map[string]any, got")

	// ErrRateScorerParamsMustIncludeField is returned for: rate scorer: params must include 'field'
	ErrRateScorerParamsMustIncludeField = errors.New("rate scorer: params must include 'field'")

	// ErrRateScorerGroundTruthMustBeMapStringAny is returned for: rate scorer: groundTruth must be map[string]any
	ErrRateScorerGroundTruthMustBeMapStringAny = errors.New("rate scorer: groundTruth must be map[string]any")

	// ErrRateScorerCaseResultMustBeMapStringAny is returned for: rate scorer: caseResult must be map[string]any
	ErrRateScorerCaseResultMustBeMapStringAny = errors.New("rate scorer: caseResult must be map[string]any")

	// ErrThresholdCheckScorerParamsMustIncludeField is returned for: threshold_check scorer: params must include 'field'
	ErrThresholdCheckScorerParamsMustIncludeField = errors.New("threshold_check scorer: params must include 'field'")

	// ErrThresholdCheckScorerCaseResultMustBeMapStringAny is returned for: threshold_check scorer: caseResult must be map[string]any
	ErrThresholdCheckScorerCaseResultMustBeMapStringAny = errors.New("threshold_check scorer: caseResult must be map[string]any")

	// ErrCannotConvert is returned for: cannot convert
	ErrCannotConvert = errors.New("cannot convert")
)
