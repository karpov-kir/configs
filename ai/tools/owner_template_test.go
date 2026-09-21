package tools_test

// The shipped owner template carries every line of the region every other tier is given.

// It lives here, and not in `ai/tools/ai-bootstrap/`, for the reason shipped_tree_test.go's cases do.
// What the installer does WITH the template belongs to that package's own suite.

import (
	"os"
	"strings"
	"testing"

	"configs/ai/tools/flavor"
)

// The template the owner tier copies and the region every other tier is given say the same thing, and
// neither can be derived from the other. Three generated lines would cost a generator and a gate unit
// to keep it honest. This case is what catches the two wordings drifting apart.

// The comparison runs over the body's lines, because the owner's copy sits under a heading and beside
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
