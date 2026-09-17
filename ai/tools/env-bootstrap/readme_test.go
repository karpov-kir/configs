package envbootstrap

// The brew list this installer holds and the one env/README.md documents cannot drift apart: adding a
// formula to the README alone would leave it documented and never installed, with every other case in
// this package still green.
//
// Read from the shipped README rather than from a fixture, so the file people actually edit is the one
// held here. The gate keys this package on it for that reason — Go's own test cache cannot see a file
// outside the module.

import (
	"os"
	"regexp"
	"slices"
	"testing"
)

const readmeFile = "../../../env/README.md"

// The README writes each one as an inline code span: `brew install neovim`, `brew install --cask
// ghostty`. Two patterns rather than one with an optional flag: `--cask` is inside the formula
// pattern.s character class, so a single pattern would file ghostty under formulae.
var (
	readmeFormula = regexp.MustCompile("`brew install ([a-z0-9-]+)`")
	readmeCask    = regexp.MustCompile("`brew install --cask ([a-z0-9-]+)`")
)

func TestTheReadmeAndThisInstallerNameTheSamePackages(t *testing.T) {
	body, err := os.ReadFile(readmeFile)
	if err != nil {
		t.Fatalf("reading %s, which is the other half of this comparison: %v", readmeFile, err)
	}
	documentedFormulae := named(readmeFormula, string(body))
	documentedCasks := named(readmeCask, string(body))

	// The control, and the load-bearing half: these needles come out of a regexp, and one that stopped
	// matching would leave both comparisons below holding a list against an empty one — a shape that
	// can only go red, never green, but only after someone reads why. Named here so the reason arrives
	// with the failure instead.
	if len(documentedFormulae) == 0 || len(documentedCasks) == 0 {
		t.Fatalf("%s documents %d formula(e) and %d cask(s), so the comparison below would run against "+
			"nothing. Either the README stopped writing them as `brew install …` code spans, or this scan "+
			"is reading the wrong file.", readmeFile, len(documentedFormulae), len(documentedCasks))
	}

	if want := sorted(formulae); !slices.Equal(documentedFormulae, want) {
		t.Errorf("%s documents formulae %v, this installer installs %v — one of the two grew a package the "+
			"other did not, and a package only the README names is one no machine ever gets",
			readmeFile, documentedFormulae, want)
	}
	if want := sorted(casks); !slices.Equal(documentedCasks, want) {
		t.Errorf("%s documents casks %v, this installer installs %v", readmeFile, documentedCasks, want)
	}
}

// Every name the pattern found, sorted and deduplicated: the README lists them one per feature and the
// order it does that in is prose, not a fact this comparison is about.
func named(pattern *regexp.Regexp, body string) []string {
	var found []string
	for _, match := range pattern.FindAllStringSubmatch(body, -1) {
		found = append(found, match[1])
	}
	return sorted(found)
}

func sorted(values []string) []string {
	unique := slices.Clone(values)
	slices.Sort(unique)
	return slices.Compact(unique)
}
