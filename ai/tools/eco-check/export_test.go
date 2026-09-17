package ecocheck

import "configs/ai/tools/shell"

const (
	FindingCap       = findingCap
	FindingNameCap   = findingNameCap
	LineWidthCap     = lineWidthCap
	SuppressedMarker = suppressedMarker
	UnshownMarker    = unshownMarker
	UnnamedClassRank = unnamedClassRank

	SharedRegionBodyCap = sharedRegionBodyCap
	FlagCallSiteCap     = flagCallSiteCap
)

const (
	SyntaxError = syntaxError

	SharedRegionHasDrifted         = sharedRegionHasDrifted
	SharedRegionNotChecked         = sharedRegionNotChecked
	SharedRegionWithoutCounterpart = sharedRegionWithoutCounterpart
	DirectionScanReadNoFiles       = directionScanReadNoFiles
	FileTooLargeToScan             = fileTooLargeToScan
	FileCouldNotBeRead             = fileCouldNotBeRead
	SkillsNotMounted               = skillsNotMounted
	SkillNotMounted                = skillNotMounted
	SkillMountedElsewhere          = skillMountedElsewhere
	MountWithoutASkill             = mountWithoutASkill

	BudgetFileRefused             = budgetFileRefused
	ScriptNamesMissingTest        = scriptNamesMissingTest
	ScriptNamesAmbiguousTest      = scriptNamesAmbiguousTest
	ScriptNamesTooManySuites      = scriptNamesTooManySuites
	BasenameNotChecked            = basenameNotChecked
	SubcommandCallSitesNotChecked = subcommandCallSitesNotChecked
	UnreadDispatch                = unreadDispatch
	FlagCallSitesNotChecked       = flagCallSitesNotChecked
	FlagScanAtItsBound            = flagScanAtItsBound
	ScriptUsageSpellingUnread     = scriptUsageSpellingUnread

	ScriptNotExecutable   = scriptNotExecutable
	StageNothingCanInvoke = stageNothingCanInvoke

	ImportRefused = importRefused

	AnyRepoNamesWorkflowFamily       = anyRepoNamesWorkflowFamily
	BareRuleIDCitation               = bareRuleIDCitation
	CitationPathIsPattern            = citationPathIsPattern
	CitationTargetNotRegular         = citationTargetNotRegular
	DanglingHomeRef                  = danglingHomeRef
	DanglingLink                     = danglingLink
	DanglingPathRef                  = danglingPathRef
	DanglingSectionRef               = danglingSectionRef
	FlagUsageDoesNotName             = flagUsageDoesNotName
	InjectListsMissingDoc            = injectListsMissingDoc
	ScriptDeclaresNoTestPosition     = scriptDeclaresNoTestPosition
	SharedLayerCitesLane             = sharedLayerCitesLane
	SharedLayerNamesLane             = sharedLayerNamesLane
	SharedLayerReachesLaneByBasename = sharedLayerReachesLaneByBasename
	SkillDirWithoutSkillFile         = skillDirWithoutSkillFile
	SkillNameDirMismatch             = skillNameDirMismatch
	SkillWithoutDescription          = skillWithoutDescription
	StandardWithoutLayer             = standardWithoutLayer
	UnreadableLayer                  = unreadableLayer
	LayerCrossingCycle               = layerCrossingCycle
	StandardsNotADirectory           = standardsNotADirectory
	StandardsPathHidden              = standardsPathHidden
	SubcommandDispatchDoesNotAccept  = subcommandDispatchDoesNotAccept
	SubcommandUsageDoesNotName       = subcommandUsageDoesNotName
	SubcommandWithNoCallSite         = subcommandWithNoCallSite
	DispatchNamesFewerFlags          = dispatchNamesFewerFlags
	UncheckableCitation              = uncheckableCitation
	UndelimitedSectionCitation       = undelimitedSectionCitation
	UnknownSkillReferenced           = unknownSkillReferenced
	MalformedSkillName               = malformedSkillName
	UnresolvableCitationPath         = unresolvableCitationPath
)

const HarnessCitationNote = harnessCitationNote

type RankTableRow struct {
	Prefix string
	Rank   int
}

func RankTable() []RankTableRow {
	rows := make([]RankTableRow, len(rankTable))
	for i, row := range rankTable {
		rows[i] = RankTableRow{Prefix: row.prefix, Rank: row.rank}
	}
	return rows
}

// The report's cut over a finding list, as the lines it would print. printFindings adds two bounds
// after this one: the 200-line cap on the report and the 500-byte cut per line. A case reasoning
// about printed output has to stay inside both.
//
// A case reaches a suppression note's counts here rather than by building a tree that floods several
// classes of one rank.
func KeptLines(findings []string) []string {
	var lines []string
	for _, kept := range keptWithinTheBudgets(shell.SortUnique(findings)) {
		lines = append(lines, kept.text)
	}
	return lines
}
