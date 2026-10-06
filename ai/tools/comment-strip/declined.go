package commentstrip

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"

	"configs/ai/tools/shell"
)

// A site a writer answered `none` stays declined while what it decided on holds: the rules, the
// declaration, the code under it and the claims it weighed. Before this record, every run offered the
// archive's claims again. Run 22 over an unchanged tree prompted two writers for 82 sites, and run 20 had
// declined them under the same rules.

// DeclinedLine ends the stderr line naming an offer the strip held back. A run counts them by it.
const DeclinedLine = " declined it, since its record holds"

// declined is one archived site a writer declined, as the archive keeps it.
type declined struct {
	Run    string `json:"run"`
	Rules  string `json:"rules"`
	Decl   string `json:"decl"`
	Span   string `json:"span"`
	Claims string `json:"claims"`
}

// declinedName is the file under the archive holding the declined sites of one source file.
func declinedName(archive, path string) string {
	return filepath.Join(archive, strings.TrimSuffix(archiveName(path, 0), "@0.facts")+".declined")
}

func readDeclined(archive, path string) []declined {
	body, err := os.ReadFile(declinedName(archive, path))
	if err != nil {
		return nil
	}
	var held []declined
	if json.Unmarshal(body, &held) != nil {
		return nil
	}
	return held
}

// claimsSum identifies the claims an archived record holds by their words. A new claim or a changed
// one at the same declaration then makes a new offer.
func claimsSum(claims string) string {
	// The set of claims, in any order and however a run sectioned them: an offer archives its site's
	// claims again under an earlier run's marker.
	var each []string
	for _, block := range strings.Split(claims, earlierMarker) {
		if claim := normalClaim(block); claim != "" && !slices.Contains(each, claim) {
			each = append(each, claim)
		}
	}
	sort.Strings(each)
	sum := sha256.Sum256([]byte(strings.Join(each, "\n")))
	return hex.EncodeToString(sum[:])[:12]
}

// declinedBy is the run that declined the record's site where that decision still holds, or "".
func declinedBy(held []declined, record archived, span, rules string) string {
	claims := claimsSum(record.claims)
	for _, d := range held {
		if sameRules(d.Rules, rules) && d.Decl == record.decl && d.Span == span && d.Claims == claims {
			return d.Run
		}
	}
	return ""
}

// OfferedRecords names the archived records of path whose claims a facts file carried, and of those the
// records holding the site's own claims, its first block. A `none` declines every claim the writer
// weighed there. A block written settles only the site's own claims: a claim offered beside them as an
// earlier run's belongs to a declaration of its own.
func OfferedRecords(archive, path, facts string) (all, own []string, err error) {
	records, err := readArchive(archive, path)
	if err != nil {
		return nil, nil, err
	}
	// A facts file opens on its site and carries a record id over each claim, and the archive keeps
	// neither, so both go before the claims are compared.
	_, body, _ := strings.Cut(facts, "\n")
	var kept []string
	// A loop round adds the review's sentence under `# code review:` and the strip a `contradicted:`
	// line, and neither is a claim the archive holds.
	for _, line := range strings.Split(body, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "# code review:" {
			break
		}
		if !strings.HasPrefix(trimmed, "# record ") && !strings.HasPrefix(trimmed, "contradicted:") {
			kept = append(kept, line)
		}
	}
	offered, first := map[string]bool{}, ""
	for n, block := range strings.Split(strings.Join(kept, "\n"), earlierMarker) {
		if claim := normalClaim(block); claim != "" {
			offered[claim] = true
			if n == 0 {
				first = claim
			}
		}
	}
	for _, record := range records {
		for n, block := range strings.Split(record.claims, earlierMarker) {
			if claim := normalClaim(block); claim != "" && offered[claim] {
				all = append(all, record.name)
				if n == 0 && claim == first {
					own = append(own, record.name)
				}
				break
			}
		}
	}
	return all, own, nil
}

// Decide records a writer's verdict on the archived records named. A decline records each against the
// code at its declaration in tree, and a written block takes its decline away. It returns how many
// records it changed.
func Decide(archive, run, tree, path string, names []string, isDeclined bool) (int, error) {
	if len(names) == 0 {
		return 0, nil
	}
	records, err := readArchive(archive, path)
	if err != nil {
		return 0, err
	}
	rules := rulesSum()
	if isDeclined && rules == "" {
		return 0, fmt.Errorf("cannot read the rules under ~/.kk-flavor, and a decline keeps the rules it was made under")
	}
	read := path
	if !filepath.IsAbs(read) {
		read = filepath.Join(tree, path)
	}
	body, err := os.ReadFile(read)
	if err != nil {
		return 0, fmt.Errorf("cannot read %s", shell.Echoable(path))
	}
	lines := shell.SplitLines(string(body))
	held := readDeclined(archive, path)
	changed := 0
	for _, record := range records {
		if !containsName(names, record.name) {
			continue
		}
		claims := claimsSum(record.claims)
		var had *declined
		kept := held[:0]
		for _, d := range held {
			if d.Decl != record.decl || d.Claims != claims {
				kept = append(kept, d)
			} else {
				d := d
				had = &d
			}
		}
		held = kept
		if !isDeclined {
			if had != nil {
				changed++
			}
			continue
		}
		at := placeRecord(lines, record, readDeclined(archive, path), rules, min(max(record.line, 1), max(len(lines), 1)))
		if at > len(lines) {
			continue
		}
		span := siteSpan(lines, at)
		if had != nil && sameRules(had.Rules, rules) && had.Span == span {
			held = append(held, *had)
			continue
		}
		held = append(held, declined{Run: run, Rules: rules, Decl: record.decl, Span: span, Claims: claims})
		changed++
	}
	if changed == 0 {
		return 0, nil
	}
	return changed, writeDeclined(archive, path, held)
}

// siteSpan is the code a site's decline was decided on. A claim whose declaration left the file sits at
// the file's level, under no code, and its decline holds on the rules and its claims only.
func siteSpan(lines []string, at int) string {
	if at == fileLevel {
		return "file-level"
	}
	return codeSum(lines, at)
}

func containsName(names []string, name string) bool {
	for _, n := range names {
		if n == name {
			return true
		}
	}
	return false
}

// Offered is one archived record a site carried to its writer, as the strip saw it: the declaration,
// the code under it in the stripped file and the claims. A writer's verdict on the site covers each. A
// facts file by itself cannot say which records it carried.
type Offered struct {
	Decl   string `json:"decl"`
	Span   string `json:"span"`
	Claims string `json:"claims"`
}

// offeredName is the file beside a site's facts naming what the site carried.
func offeredName(facts string) string { return strings.TrimSuffix(facts, ".facts") + ".offered" }

// offeredAt is what one site carries. That is every archived record held at its line, and the record
// this run archives for the site, holding its own claims and the earlier ones.
func offeredAt(records []archived, held map[string]int, s site, record, stripped string) []Offered {
	span := siteSpan(shell.SplitLines(stripped), s.line)
	var out []Offered
	if _, claims, _ := strings.Cut(record, "\n"); normalClaim(claims) != "" {
		out = append(out, Offered{Decl: s.decl, Span: span, Claims: claimsSum(claims)})
	}
	for _, r := range records {
		if at, found := held[r.name]; found && at == s.line {
			out = append(out, Offered{Decl: r.decl, Span: span, Claims: claimsSum(r.claims)})
		}
	}
	return out
}

func writeOffered(dir, facts string, offered []Offered) error {
	body, err := json.MarshalIndent(offered, "", " ")
	if err != nil {
		return err
	}
	path := filepath.Join(dir, offeredName(facts))
	if err := os.WriteFile(path, append(body, '\n'), 0o644); err != nil {
		return fmt.Errorf("cannot write %s", shell.Echoable(path))
	}
	return nil
}

// ReadOffered is what a site's facts file carried, or nil for a run before the strip wrote it.
func ReadOffered(facts string) []Offered {
	body, err := os.ReadFile(offeredName(facts))
	if err != nil {
		return nil
	}
	var out []Offered
	if json.Unmarshal(body, &out) != nil {
		return nil
	}
	return out
}

// DecideOffered records a verdict on what a site carried. A decline records each under the rules now,
// and a written block takes their declines away. It returns how many records changed, and leaves the
// file as it stands where no record changed.
func DecideOffered(archive, run, path string, offered []Offered, isDeclined bool) (int, error) {
	if len(offered) == 0 {
		return 0, nil
	}
	rules := rulesSum()
	if isDeclined && rules == "" {
		return 0, fmt.Errorf("cannot read the rules under ~/.kk-flavor, and a decline keeps the rules it was made under")
	}
	held := readDeclined(archive, path)
	changed := 0
	for _, o := range offered {
		entry := declined{Run: run, Rules: rules, Decl: o.Decl, Span: o.Span, Claims: o.Claims}
		var had *declined
		kept := held[:0]
		for _, d := range held {
			if d.Decl != o.Decl || d.Claims != o.Claims {
				kept = append(kept, d)
			} else {
				d := d
				had = &d
			}
		}
		held = kept
		switch {
		case isDeclined:
			held = append(held, entry)
			// The same decline recorded again leaves the record as it stands, whichever run recorded it first.
			if had == nil || !sameRules(had.Rules, rules) || had.Span != o.Span {
				changed++
			} else {
				held[len(held)-1] = *had
			}
		case had != nil:
			changed++
		}
	}
	if changed == 0 {
		return 0, nil
	}
	return changed, writeDeclined(archive, path, held)
}

func writeDeclined(archive, path string, held []declined) error {
	out, err := json.MarshalIndent(held, "", " ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(declinedName(archive, path), append(out, '\n'), 0o644); err != nil {
		return fmt.Errorf("cannot write %s", shell.Echoable(declinedName(archive, path)))
	}
	return nil
}

// RecordsAtSite names the archived records of path the strip would hold at line, in the stripped file a
// round read. A verdict without a record of what its site carried covers these records only.
func RecordsAtSite(archive, path string, stripped []string, line int) ([]string, error) {
	records, err := readArchive(archive, path)
	if err != nil {
		return nil, err
	}
	declines, rules := readDeclined(archive, path), rulesSum()
	var names []string
	for _, record := range records {
		if placeRecord(stripped, record, declines, rules, min(max(record.line, 1), max(len(stripped), 1))) == line {
			names = append(names, record.name)
		}
	}
	return names, nil
}

// RulesOfRun is the rules sum the archive's written blocks of run carry, or "" where it holds none.
func RulesOfRun(archive, run string) string {
	entries, _ := os.ReadDir(archive)
	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), ".written") {
			continue
		}
		body, _ := os.ReadFile(filepath.Join(archive, e.Name()))
		var held []written
		if json.Unmarshal(body, &held) != nil {
			continue
		}
		for _, w := range held {
			if w.Run == run && w.Rules != "" {
				return w.Rules
			}
		}
	}
	return ""
}

// heldDecline is the run whose decline holds the record at line `at`. A record whose declaration left
// the file reads at the file level. A decline of its claims on that declaration then holds it at any
// span, since the code that decline weighed is gone. Run 26 declined
// two member records, the lane folded the member into another type, and run 28 offered them again.
func heldDecline(held []declined, record archived, lines []string, at int, rules string) string {
	if run := declinedBy(held, record, siteSpan(lines, at), rules); run != "" || at != fileLevel || record.decl == "" {
		return run
	}
	for _, line := range lines {
		if strings.TrimSpace(line) == record.decl {
			return ""
		}
	}
	claims := claimsSum(record.claims)
	for _, d := range held {
		if sameRules(d.Rules, rules) && d.Decl == record.decl && d.Claims == claims {
			return d.Run
		}
	}
	return ""
}
