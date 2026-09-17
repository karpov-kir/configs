package tools_test

// The shipped owner template, held against the region every other tier is given.
//
// It lives here rather than beside the installer because ai/owner-instructions.md is outside the
// module: Go keys a package's test cache on the module, so a case that opened it from
// `ai/tools/ai-bootstrap/` would answer `ok (cached)` over a template that had changed underneath the
// run. What the installer does WITH the template is that package's own suite's.

import (
	"os"
	"strings"
	"testing"

	"kk-flavor/tools/flavor"
)

// The template the owner tier copies and the region every other tier is given say the same thing and
// cannot be derived from one another — generating three lines would cost a generator and a gate unit
// to keep it honest. This is what catches the wording drifting apart.
//
// Compared as the body's lines, not as a whole file: the owner's copy sits under a heading and beside
// prose the fenced copy has no business carrying.
func TestTheShippedOwnerTemplateCarriesEveryLineOfTheRegionBody(t *testing.T) {
	const shipped = repoRoot + "/ai/owner-instructions.md"
	body, err := os.ReadFile(shipped)
	if err != nil {
		t.Fatalf("reading %s, which is the other half of this comparison: %v", shipped, err)
	}
	lines := strings.Split(flavor.RegionBody, "\n")
	if len(lines) < 2 {
		t.Fatalf("the region body is %d line(s), so this comparison would assert almost nothing", len(lines))
	}
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		if !strings.Contains(string(body), line) {
			t.Errorf("%s does not carry %q, which every other tier is given — the two wordings have drifted "+
				"apart, and an owner and a colleague are now reading different instructions", shipped, line)
		}
	}
}
