package aibootstrap

// The formulae this installer holds and the ones ai/README.md documents cannot drift apart: adding one
// to the README alone would leave it documented and never installed, with every other case in this
// package still green.
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

const readmeFile = "../../README.md"

// The README writes each one as an inline code span: `brew install jq`.
var readmeFormula = regexp.MustCompile("`brew install ([a-z0-9-]+)`")

func TestTheReadmeAndThisInstallerNameTheSameFormulae(t *testing.T) {
	body, err := os.ReadFile(readmeFile)
	if err != nil {
		t.Fatalf("reading %s, which is the other half of this comparison: %v", readmeFile, err)
	}
	var documented []string
	for _, match := range readmeFormula.FindAllStringSubmatch(string(body), -1) {
		documented = append(documented, match[1])
	}
	slices.Sort(documented)
	documented = slices.Compact(documented)

	// The control, and the load-bearing half: this needle comes out of a regexp, and one that stopped
	// matching would leave the comparison below holding a list against an empty one — a shape that can
	// only go red, and only once someone reads why. Named here so the reason arrives with the failure.
	if len(documented) == 0 {
		t.Fatalf("%s documents no formula at all, so the comparison below would run against nothing. "+
			"Either the README stopped writing them as `brew install …` code spans, or this scan is "+
			"reading the wrong file.", readmeFile)
	}

	var installed []string
	for _, formula := range formulae {
		installed = append(installed, formula.name)
	}
	slices.Sort(installed)
	if !slices.Equal(documented, installed) {
		t.Errorf("%s documents formulae %v, this installer installs %v — one of the two grew a formula the "+
			"other did not, and one only the README names is one no machine ever gets",
			readmeFile, documented, installed)
	}
}
