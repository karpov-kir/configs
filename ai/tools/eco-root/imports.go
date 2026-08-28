package ecoroot

import (
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"

	"kk-flavor/tools/shell"
)

// The `@import` half of a checkout: what the budget files declare, and what the installed mount
// resolves those declarations to. It lives with the root rather than among the shell primitives
// because every answer here is a fact about *this* checkout — whether it is the installed one, which
// CLAUDE.md declared the name, which mount the file loads from — and a caller that could supply those
// separately could resolve a set the other tool cannot reproduce.

var (
	backtickSpan = regexp.MustCompilePOSIX("`[^`]*`")
	// Any extension counts, but one is required: that and the non-word character before the `@` are
	// what keep `@param`, a package scope and an email address out of the import list.
	importToken = regexp.MustCompilePOSIX(`[^A-Za-z0-9_]@[~A-Za-z0-9._/-]+\.[A-Za-z0-9]+`)
)

// Every `@name.ext` outside a fence and outside backticks, across the given files, byte-sorted and
// deduplicated. Two bounds keep it linear in text the tree chose: a field length cap, and a match cap
// within one field.
func importsIn(read ReadLines, files []string) []string {
	var found []string
	for _, file := range files {
		lines, err := read(file)
		if err != nil {
			continue
		}
		inFence := false
		for _, line := range lines {
			if shell.IsFenceDelimiter(line) {
				inFence = !inFence
				continue
			}
			if inFence {
				continue
			}
			for _, field := range shell.SplitFields(backtickSpan.ReplaceAllString(line, " ")) {
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
	return shell.SortUnique(found)
}

// An import loads from beside the *installed* copy of the file carrying it, so `@RTK.md` in CLAUDE.md
// is `~/.claude/RTK.md`. That file is **not** one this repo forgot: the rtk installer puts it there
// and verifies it, so moving it into the tree fights the installer.
//
// Only CLAUDE.md's own imports resolve here — an inject.md import loads from `~/.kk-flavor/`, so
// resolving one here would count whatever file shares the name.
type importMount struct {
	home        string
	isInstalled bool
	// Depth 1: an import nested inside a resolved file is neither counted nor named.
	declared map[string]bool
}

// The two facts the resolver needs, both read off the root: whether this checkout is the installed
// one, and which imports its own CLAUDE.md declares. Unexported, and reachable only through
// Root.ResolveImports, because a caller free to pass a different pair would resolve a different set
// and report a budget the other tool cannot reproduce.
func (r Root) newImportMount(read ReadLines) importMount {
	declared := map[string]bool{}
	claudeMd := shell.Join(r.named, "CLAUDE.md")
	if !shell.IsSymlink(claudeMd) && shell.IsRegularFile(claudeMd) && isReadable(claudeMd) {
		for _, name := range importsIn(read, []string{claudeMd}) {
			declared[name] = true
		}
	}
	return importMount{home: r.home, isInstalled: r.IsInstalled(), declared: declared}
}

// resolve returns the file an import name loads from, or the reason it was refused. A reason is
// carried only for the shapes nothing legitimate produces — a traversal, a symlink planted at the
// mount path, a file present and deliberately unreadable. An import simply absent from the mount, a
// checkout that isn't the installed one, and a subdirectory import this resolver does not handle are
// the ordinary cases: they stay quiet names in the note.
func (m importMount) resolve(name string) (target, refusal string) {
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
	mounted := shell.Join(shell.Join(m.home, ".claude"), name)
	if shell.IsSymlink(mounted) {
		return "", "a symlink at the mount"
	}
	if !shell.IsRegularFile(mounted) {
		return "", ""
	}
	if !isReadable(mounted) {
		return "", "unreadable at the mount"
	}
	return mounted, ""
}

// ResolveImports scans the given files for `@import` names, hands each one the mount holds to
// Resolved, each name of a shape nothing legitimate produces to Refused, and returns the rest for the
// caller to name in its note. Scanning and mounting are one step because their order is not the
// caller's to get wrong: a mount built against a different root, or built before the file list is
// complete, resolves a set the other tool cannot reproduce.
//
// Attempts are capped, and past the cap every remaining name goes to the returned list rather than
// dropping out silently. Nothing is carried over from the last name the resolver examined: past the
// cap it is not called, so its own reset never runs and the last examined name's reason would
// otherwise be reported against a name nothing looked at.
//
// The shell version accumulated the leftovers in a scratch file, because `s="$s$name"` re-copies
// everything gathered so far on every name and that is quadratic in a count the attacker picks. An
// append to a slice is not, so the scratch file and its mktemp failure path are gone with it.
func (r Root) ResolveImports(scan ImportScan) []string {
	mount := r.newImportMount(scan.Read)
	var uncounted []string
	attempts := 0
	for _, name := range importsIn(scan.Read, scan.Files) {
		if name == "" {
			continue
		}
		target, refusal := "", ""
		if attempts < 64 {
			attempts++
			target, refusal = mount.resolve(name)
		}
		if target != "" {
			scan.Resolved(target)
			continue
		}
		if refusal != "" {
			scan.Refused(name, refusal)
		}
		uncounted = append(uncounted, name)
	}
	return uncounted
}

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
		joined.WriteString(shell.CutBytes(shell.Oneline(name), 60))
		joined.WriteString(" ")
	}
	names := strings.TrimSuffix(shell.CutBytes(joined.String(), 200), " ")
	if len(uncounted) > 10 {
		names += " … and " + strconv.Itoa(len(uncounted)-10) + " more"
	}
	return names
}
