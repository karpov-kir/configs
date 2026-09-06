package ecocheck

import "kk-flavor/tools/shell"

// What the suite next door needs from inside this package. Bind a value here rather than retyping it
// in a case: a copy goes stale the next time the original is reworded, and the case stays green.
const (
	FindingCap       = findingCap
	FindingNameCap   = findingNameCap
	SuppressedMarker = suppressedMarker
	UnshownMarker    = unshownMarker
	UnnamedClassRank = unnamedClassRank

	SharedRegionBodyCap = sharedRegionBodyCap
)

// The finding heads, which are also rankTable's rows: a case matching on one is asserting about the
// kind that row names.
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

	ScriptNotExecutable = scriptNotExecutable

	ImportRefused = importRefused

	AnyRepoNamesWorkflowFamily       = anyRepoNamesWorkflowFamily
	BareRuleIDCitation               = bareRuleIDCitation
	CitationPathIsPattern            = citationPathIsPattern
	CitationTargetNotRegular         = citationTargetNotRegular
	DanglingHomeRef                  = danglingHomeRef
	DanglingLink                     = danglingLink
	DanglingPathRef                  = danglingPathRef
	DanglingSectionRef               = danglingSectionRef
	InjectListsMissingDoc            = injectListsMissingDoc
	ScriptDeclaresNoTestPosition     = scriptDeclaresNoTestPosition
	SharedLayerCitesLane             = sharedLayerCitesLane
	SharedLayerNamesLane             = sharedLayerNamesLane
	SharedLayerReachesLaneByBasename = sharedLayerReachesLaneByBasename
	SkillDirWithoutSkillFile         = skillDirWithoutSkillFile
	SkillNameDirMismatch             = skillNameDirMismatch
	SkillWithoutDescription          = skillWithoutDescription
	SubcommandDispatchDoesNotAccept  = subcommandDispatchDoesNotAccept
	SubcommandUsageDoesNotName       = subcommandUsageDoesNotName
	SubcommandWithNoCallSite         = subcommandWithNoCallSite
	UncheckableCitation              = uncheckableCitation
	UndelimitedSectionCitation       = undelimitedSectionCitation
	UnknownSkillReferenced           = unknownSkillReferenced
	UnresolvableCitationPath         = unresolvableCitationPath
)

// The note a citation finding against a test harness ends on. A case spelling a fragment of it out
// asserts nothing once the note is reworded around that fragment, and the case asserting the note is
// ABSENT is the one that goes green saying so.
const HarnessCitationNote = harnessCitationNote

// One row of the rank table, exported so a case can hold the properties a row needs without copying
// the table into the suite.
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
