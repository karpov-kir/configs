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
func declinedBy(held []declined, record archived, span string) string {
	rules, claims := rulesSum(), claimsSum(record.claims)
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
	for _, line := range strings.Split(body, "\n") {
		if !strings.HasPrefix(strings.TrimSpace(line), "# record ") {
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
		kept := held[:0]
		for _, d := range held {
			if d.Decl != record.decl || d.Claims != claims {
				kept = append(kept, d)
			}
		}
		removed := len(kept) != len(held)
		held = kept
		if !isDeclined {
			if removed {
				changed++
			}
			continue
		}
		at := declarationLine(lines, record.decl, record.line, min(max(record.line, 1), max(len(lines), 1)))
		if at > len(lines) {
			continue
		}
		held = append(held, declined{Run: run, Rules: rules, Decl: record.decl, Span: siteSpan(lines, at), Claims: claims})
		changed++
	}
	out, err := json.MarshalIndent(held, "", " ")
	if err != nil {
		return 0, err
	}
	if err := os.WriteFile(declinedName(archive, path), append(out, '\n'), 0o644); err != nil {
		return 0, fmt.Errorf("cannot write %s", shell.Echoable(declinedName(archive, path)))
	}
	return changed, nil
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
