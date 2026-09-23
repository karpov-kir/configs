package commentread

import (
	"os"
	"testing"
)

func TestParse(t *testing.T) {
	for _, tc := range []struct {
		raw    string
		cannot bool
		parsed bool
	}{
		{"reads: `getFormatClaim` returns Accepted when either answer is Accepted.", false, true},
		{"cannot say: no tie", true, true},
		{"Cannot say: \"fresh history\"", true, true},
		{"reads: something\ncannot say: \"alone\"", true, true},
		{"The line returns a claim.", false, false},
	} {
		got := Parse(tc.raw)
		if got.CannotSay != tc.cannot || got.Parsed != tc.parsed {
			t.Errorf("Parse(%q) = %+v, want cannot %v parsed %v", tc.raw, got, tc.cannot, tc.parsed)
		}
	}
}

func TestVerdict(t *testing.T) {
	yes, no, lost := Answer{CannotSay: true, Parsed: true}, Answer{Parsed: true}, Answer{}
	if cannot, n := Verdict([]Answer{yes, no, yes, lost}); !cannot || n != 3 {
		t.Fatalf("two of three cannot say reads as %v over %d", cannot, n)
	}
	if cannot, _ := Verdict([]Answer{yes, no}); !cannot {
		t.Fatal("a tie reads as the reader restating the block")
	}
	if cannot, n := Verdict([]Answer{lost}); cannot || n != 0 {
		t.Fatal("a roll that did not answer counts toward a verdict")
	}
}

// The labelled file parses, and every site in it carries one line of code under its block.
func TestTheLabelledSitesParse(t *testing.T) {
	raw, err := os.ReadFile(labelledPath)
	if err != nil {
		t.Fatal(err)
	}
	sites, err := ParseSites(string(raw))
	if err != nil {
		t.Fatal(err)
	}
	if len(sites) != labelledCount {
		t.Fatalf("%d labelled sites, and the bar was fixed over %d", len(sites), labelledCount)
	}
}
