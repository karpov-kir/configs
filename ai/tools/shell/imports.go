package shell

import (
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"
)

// ReadLines is how a caller hands this package a file: its lines, or an error that means "skip it".
// It is a parameter rather than a call to os.ReadFile because the two ports read a file under
// different bounds, and which bound applies is the caller's decision, not this package's.
type ReadLines func(path string) ([]string, error)

var (
	// The link form `grep -oE '\]\([^)#]+\)'` matched, with the `sed 's/^](//; s/)$//'` behind it.
	// Which *block* of a file it is applied to is not shared: check.sh reads a sed range, stats.sh an
	// awk flag, and the two select different boundary lines.
	linkTarget   = regexp.MustCompilePOSIX(`\]\([^)#]+\)`)
	backtickSpan = regexp.MustCompilePOSIX("`[^`]*`")
	// Any extension counts, but one is required: that and the non-word character before the `@` are
	// what keep `@param`, a package scope and an email address out of the import list.
	importToken = regexp.MustCompilePOSIX(`[^A-Za-z0-9_]@[~A-Za-z0-9._/-]+\.[A-Za-z0-9]+`)
)

// LinkTargets is every `](target)` on one line, the parentheses stripped.
func LinkTargets(line string) []string {
	var targets []string
	for _, match := range linkTarget.FindAllString(line, -1) {
		targets = append(targets, strings.TrimSuffix(strings.TrimPrefix(match, "]("), ")"))
	}
	return targets
}

// --- shared:import-scan ---
// Every `@name.ext` outside a fence and outside backticks, across the given files, byte-sorted and
// deduplicated. Two bounds keep it linear in text the tree chose: a field length cap, and a match cap
// within one field.
func ImportsIn(read ReadLines, files []string) []string {
	var found []string
	for _, file := range files {
		lines, err := read(file)
		if err != nil {
			continue
		}
		inFence := false
		for _, line := range lines {
			if strings.HasPrefix(line, "```") {
				inFence = !inFence
				continue
			}
			if inFence {
				continue
			}
			for _, field := range SplitFields(backtickSpan.ReplaceAllString(line, " ")) {
				if len(field) > 4096 {
					continue
				}
				// Prefixed with a space so the leading non-word character the pattern requires has
				// something to match at the head of the field.
				token := " " + field
				for hits := 0; hits < 64; hits++ {
					at := importToken.FindStringIndex(token)
					if at == nil {
						break
					}
					// The boundary the pattern requires before the `@` is one *rune*, one to four
					// bytes wide. Cutting a fixed two carries the tail of a multi-byte rune into the
					// name — `é@alpha.md` yields `@alpha.md` and `—@beta.md` a name starting
					// mid-rune, both onto the census line that prints on a clean run, and both away
					// from what the shell's byte-wise awk extracts.
					_, boundary := utf8.DecodeRuneInString(token[at[0]:])
					found = append(found, token[at[0]+boundary+1:at[1]])
					token = token[at[1]:]
				}
			}
		}
	}
	return SortUnique(found)
}

// --- end shared:import-scan ---

// --- shared:import-at-mount ---
// An import loads from beside the *installed* copy of the file carrying it, so `@RTK.md` in CLAUDE.md
// is `~/.claude/RTK.md`. That file is **not** one this repo forgot: the rtk installer puts it there
// and verifies it, so moving it into the tree fights the installer.
//
// Only CLAUDE.md's own imports resolve here — an inject.md import loads from `~/.kk-flavor/`, so
// resolving one here would count whatever file shares the name. Resolution also needs this checkout
// to be the installed one, or a branch someone else wrote names files in the invoking user's real
// `~/.claude/` and folds their sizes into a number it also authored. Canonicalising follows a
// symlinked *directory*, so refusing a symlinked `$root/kk-flavor` is what stops a branch committing
// one to the real install and opening that gate.
type ImportMount struct {
	home        string
	isInstalled bool
	// Depth 1: an import nested inside a resolved file is neither counted nor named.
	declared map[string]bool
}

// NewImportMount reads the two facts the resolver needs: whether flavorInRoot is the directory $HOME
// mounts as `.kk-flavor`, and which imports claudeMd itself declares.
func NewImportMount(home, flavorInRoot, claudeMd string, read ReadLines) ImportMount {
	isInstalled := home != "" && !IsSymlink(flavorInRoot) &&
		CanonicalDir(flavorInRoot) != "" &&
		CanonicalDir(Join(home, ".kk-flavor")) == CanonicalDir(flavorInRoot)

	declared := map[string]bool{}
	if !IsSymlink(claudeMd) && IsRegularFile(claudeMd) && isReadable(claudeMd) {
		for _, name := range ImportsIn(read, []string{claudeMd}) {
			declared[name] = true
		}
	}
	return ImportMount{home: home, isInstalled: isInstalled, declared: declared}
}

// IsInstalled reports whether this checkout is the installed one — the same gate that decides
// whether skills mounted from outside the tree can be told apart from the tree's own.
func (m ImportMount) IsInstalled() bool { return m.isInstalled }

// resolve returns the file an import name loads from, or the reason it was refused. A reason is
// carried only for the shapes nothing legitimate produces — a traversal, a symlink planted at the
// mount path, a file present and deliberately unreadable. An import simply absent from the mount, a
// checkout that isn't the installed one, and a subdirectory import this resolver does not handle are
// the ordinary cases: they stay quiet names in the note.
func (m ImportMount) resolve(name string) (target, refusal string) {
	if !m.isInstalled || name == "" {
		return "", ""
	}
	// Bare filenames only — `@../../.ssh/id_rsa` must not resolve. `@dir/file.md` is a legitimate
	// import form, so a plain subdirectory name is refused here too, but quietly: reporting it would
	// take an honest run to exit non-zero.
	switch {
	case strings.HasPrefix(name, "~"), strings.HasPrefix(name, "/"),
		strings.HasPrefix(name, "../"), strings.Contains(name, "/../"), strings.HasSuffix(name, "/.."):
		return "", "a traversal, not a bare filename"
	case strings.Contains(name, "/"):
		return "", ""
	}
	if !m.declared[name] {
		return "", ""
	}
	mounted := Join(Join(m.home, ".claude"), name)
	if IsSymlink(mounted) {
		return "", "a symlink at the mount"
	}
	if !IsRegularFile(mounted) {
		return "", ""
	}
	if !isReadable(mounted) {
		return "", "unreadable at the mount"
	}
	return mounted, ""
}

// --- end shared:import-at-mount ---

// --- shared:import-resolution ---
// ResolveImports hands each name the mount holds to resolved, each name of a shape nothing
// legitimate produces to refused, and returns the rest for the caller to name in its note. Attempts
// are capped, and past the cap every remaining name goes to that list rather than dropping out
// silently. Nothing is carried over from the last name the resolver examined: past the cap it is not
// called, so its own reset never runs and the last examined name's reason would otherwise be reported
// against a name nothing looked at.
//
// The shell version accumulated the leftovers in a scratch file, because `s="$s$name"` re-copies
// everything gathered so far on every name and that is quadratic in a count the attacker picks. An
// append to a slice is not, so the scratch file and its mktemp failure path are gone with it.
func ResolveImports(names []string, mount ImportMount, resolved func(target string), refused func(name, reason string)) []string {
	var uncounted []string
	attempts := 0
	for _, name := range names {
		if name == "" {
			continue
		}
		target, refusal := "", ""
		if attempts < 64 {
			attempts++
			target, refusal = mount.resolve(name)
		}
		if target != "" {
			resolved(target)
			continue
		}
		if refusal != "" {
			refused(name, refusal)
		}
		uncounted = append(uncounted, name)
	}
	return uncounted
}

// --- end shared:import-resolution ---

// --- shared:import-cap ---
// UncountedNames is the naming half of the uncounted-import note: capped in bytes as well as in
// entries, because that note rides the exit-0 path, so an uncapped list prints attacker-chosen text
// under a clean report. The count stays exact and is the caller's to print; only the naming is
// trimmed here.
func UncountedNames(uncounted []string) string {
	shown := uncounted
	if len(shown) > 10 {
		shown = shown[:10]
	}
	var joined strings.Builder
	for _, name := range shown {
		// Sanitised like every other name a message echoes, even though the import pattern's charset
		// admits no control byte today: a scan added later that widens that charset must not reopen
		// the injection here.
		joined.WriteString(CutBytes(Oneline(name), 60))
		joined.WriteString(" ")
	}
	names := strings.TrimSuffix(CutBytes(joined.String(), 200), " ")
	if len(uncounted) > 10 {
		names += " … and " + strconv.Itoa(len(uncounted)-10) + " more"
	}
	return names
}

// --- end shared:import-cap ---
