// The two installers' package lists, held against the two READMEs that document them. Adding a
// formula to a README alone would leave it documented and never installed, with every other case in
// either installer's package still green.
//
// Read from the shipped READMEs rather than from a fixture, so the files people actually edit are the
// ones held here. They live in this package because both are outside the module: Go keys a package's
// test cache on the module, so a case that opened one from `ai/tools/<installer>/` would answer
// `ok (cached)` over a README that had changed underneath the run.
package tools_test

import (
	"os"
	"regexp"
	"slices"
	"testing"

	aibootstrap "kk-flavor/tools/ai-bootstrap"
	envbootstrap "kk-flavor/tools/env-bootstrap"
)

const (
	aiReadme  = repoRoot + "/ai/README.md"
	envReadme = repoRoot + "/env/README.md"
)

// Both READMEs write each package as an inline code span: `brew install neovim`, `brew install --cask
// ghostty`. Two patterns rather than one with an optional flag: `--cask` is inside the formula
// pattern's character class, so a single pattern would file ghostty under formulae.
var (
	readmeFormula = regexp.MustCompile("`brew install ([a-z0-9-]+)`")
	readmeCask    = regexp.MustCompile("`brew install --cask ([a-z0-9-]+)`")
)

func TestTheAiReadmeAndItsInstallerNameTheSameFormulae(t *testing.T) {
	body := readShipped(t, aiReadme)
	documented := named(readmeFormula, body)

	// The control, and the load-bearing half: this needle comes out of a regexp, and one that stopped
	// matching would leave the comparison below holding a list against an empty one — a shape that can
	// only go red, and only once someone reads why. Named here so the reason arrives with the failure.
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
	body := readShipped(t, envReadme)
	documentedFormulae := named(readmeFormula, body)
	documentedCasks := named(readmeCask, body)

	// The same control, over both lists: these needles come out of a regexp, and one that stopped
	// matching would leave both comparisons below holding a list against an empty one — a shape that
	// can only go red, never green, but only after someone reads why. Named here so the reason arrives
	// with the failure instead.
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

// Every name the pattern found, sorted and deduplicated: a README lists them one per feature and the
// order it does that in is prose, not a fact either comparison is about.
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

// One half of a comparison, refused loudly where it cannot be read: a case that carried on would hold
// the installer's list against an empty string and report it as drift.
func readShipped(t *testing.T, path string) string {
	t.Helper()
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s, which is the other half of this comparison: %v", path, err)
	}
	return string(body)
}
