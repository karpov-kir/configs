// The two installers' package lists and the two READMEs that document them carry the same names.
// Adding a formula to a README alone leaves it documented and never installed, with every other case
// in either installer's package still green.

// These cases read the shipped READMEs, so the files people actually edit are the ones held here. They
// live in this package, and not in `ai/tools/<installer>/`, for the reason shipped_tree_test.go's
// cases do.
package tools_test

import (
	"regexp"
	"slices"
	"testing"

	aibootstrap "configs/ai/tools/ai-bootstrap"
	envbootstrap "configs/ai/tools/env-bootstrap"
)

const (
	aiReadme  = repoRoot + "/ai/README.md"
	envReadme = repoRoot + "/env/README.md"
)

// Both READMEs write each package as an inline code span: `brew install neovim`, `brew install --cask
// ghostty`. There are two patterns because `--cask` sits inside the formula pattern's character class,
// and one pattern with an optional flag files ghostty under formulae.
var (
	readmeFormula = regexp.MustCompile("`brew install ([a-z0-9-]+)`")
	readmeCask    = regexp.MustCompile("`brew install --cask ([a-z0-9-]+)`")
)

func TestTheAiReadmeAndItsInstallerNameTheSameFormulae(t *testing.T) {
	body := readFile(t, aiReadme)
	documented := named(readmeFormula, body)

	// The control half of the case. This needle comes out of a regexp, and one that stopped matching
	// leaves slices.Equal holding a list against an empty one. That shape can only go red, and only once
	// someone reads why, so the reason is named here and arrives with the failure.
	if len(documented) == 0 {
		t.Fatalf("%s documents no formula at all, so the comparison below would run against nothing. "+
			"Either the README stopped writing them as `brew install …` code spans, or this scan is "+
			"reading the wrong file.", aiReadme)
	}

	if installed := sortedUnique(aibootstrap.FormulaNames()); !slices.Equal(documented, installed) {
		t.Errorf("%s documents formulae %v, this installer installs %v — one of the two grew a formula the "+
			"other did not, and one only the README names is one no machine ever gets",
			aiReadme, documented, installed)
	}
}

func TestTheEnvReadmeAndItsInstallerNameTheSamePackages(t *testing.T) {
	body := readFile(t, envReadme)
	documentedFormulae := named(readmeFormula, body)
	documentedCasks := named(readmeCask, body)

	// The same control, over both lists. These needles come out of a regexp, and one that stopped
	// matching leaves both slices.Equal calls holding a list against an empty one. That shape can only
	// go red, and only once someone reads why, so the reason is named here.
	if len(documentedFormulae) == 0 || len(documentedCasks) == 0 {
		t.Fatalf("%s documents %d formula(e) and %d cask(s), so the comparison below would run against "+
			"nothing. Either the README stopped writing them as `brew install …` code spans, or this scan "+
			"is reading the wrong file.", envReadme, len(documentedFormulae), len(documentedCasks))
	}

	if want := sortedUnique(envbootstrap.FormulaNames()); !slices.Equal(documentedFormulae, want) {
		t.Errorf("%s documents formulae %v, this installer installs %v — one of the two grew a package the "+
			"other did not, and a package only the README names is one no machine ever gets",
			envReadme, documentedFormulae, want)
	}
	if want := sortedUnique(envbootstrap.CaskNames()); !slices.Equal(documentedCasks, want) {
		t.Errorf("%s documents casks %v, this installer installs %v", envReadme, documentedCasks, want)
	}
}

// Returns every name the pattern found, sorted and deduplicated. A README lists them one per feature,
// and neither comparison is about the order it does that in.
func named(pattern *regexp.Regexp, body string) []string {
	var found []string
	for _, match := range pattern.FindAllStringSubmatch(body, -1) {
		found = append(found, match[1])
	}
	return sortedUnique(found)
}

func sortedUnique(values []string) []string {
	unique := slices.Clone(values)
	slices.Sort(unique)
	return slices.Compact(unique)
}
