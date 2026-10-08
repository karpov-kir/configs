package commentpass

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// A declaration's purpose often lives where it is called: a caller runs it for a reason this file
// never states, and that reason is often stated one call further out. So the call carries, for each
// exported declaration in the material, a few of its call sites found in the repository, and a few
// call sites of the declarations that enclose those. Each comes with the comment on its enclosing
// declaration. A first-level site shows that declaration's whole body where it is short, and every
// other site the lines around it. They are context, never material: the call decides nothing in them.

const (
	// sitesPerName bounds the call sites shown for one declaration.
	sitesPerName = 3
	// linesAbove and linesBelow are the context around a call site, eight lines with the site.
	linesAbove = 4
	linesBelow = 3
	// maxBodyLines is the longest enclosing body a first-level site shows whole.
	maxBodyLines = 30
	// maxEdgeLines bounds the comment shown above a window, so a long header or a commented-out block
	// does not take the section.
	maxEdgeLines = 10
	// maxCallersBytes bounds the whole section, so a widely called file does not crowd out its own text.
	maxCallersBytes = 20_000
)

// reExportedTS is an exported TypeScript or JavaScript declaration, with its name.
var reExportedTS = regexp.MustCompile(`^\s*export\s+(?:default\s+)?(?:declare\s+)?(?:abstract\s+)?(?:async\s+)?` +
	`(?:function\*?|class|const\s+enum|const|let|var|type|interface|enum)\s+([A-Za-z_$][\w$]*)`)

// reExportedGo is an exported Go function, method or type, with its name.
var reExportedGo = regexp.MustCompile(`^(?:func\s+(?:\([^)]*\)\s*)?|type\s+)([A-Z]\w*)`)

// reImport is a line that names a declaration without calling it.
var reImport = regexp.MustCompile(`^\s*(?:import\b|export\s+(?:\*|\{[^}]*\})\s+from\b|\} from\b|[\w$,\s]*\} from\b)`)

// exportedName is the name a declaration line exports, or empty.
func exportedName(path, line string) string {
	re := reExportedTS
	if filepath.Ext(path) == ".go" {
		re = reExportedGo
	}
	if m := re.FindStringSubmatch(line); m != nil {
		return m[1]
	}
	return ""
}

// callersSection is the context the call carries on who calls this file's exported material, or
// empty when nothing outside the file calls it.
func callersSection(top, path string, lines []string, m material) string {
	var names []string
	seen := map[string]bool{}
	add := func(line int) {
		if line < 1 || line > len(lines) {
			return
		}
		if name := exportedName(path, lines[line-1]); name != "" && !seen[name] {
			seen[name] = true
			names = append(names, name)
		}
	}
	for _, c := range m.candidates {
		add(c.decl)
	}
	for _, p := range m.places {
		add(p.line)
	}
	// Every name's direct callers come before any caller one call further out, so a file with several
	// exports keeps its later exports' direct callers inside the cap.
	var first, second []string
	for _, name := range names {
		direct, further := callSites(top, path, name)
		first, second = append(first, direct...), append(second, further...)
	}
	var b strings.Builder
	for _, site := range append(first, second...) {
		if b.Len()+len(site) > maxCallersBytes {
			continue
		}
		b.WriteString(site)
	}
	if b.Len() == 0 {
		return ""
	}
	return "\nCallers of this file's exported declarations, as context only. Decide nothing in them, and " +
		"use them for what a caller relies on:\n" + b.String()
}

// site is one place a name is used: the file's lines and the line, counted from 1.
type site struct {
	file  string
	lines []string
	at    int
}

// callSites sets out up to sitesPerName places outside path that use name, each with the body or the
// lines around it, and up to sitesPerName places that use the declarations enclosing those.
func callSites(top, path, name string) (direct, further []string) {
	first := findSites(top, name, func(file string, _ int) bool { return file == path })
	var second []site
	followed := map[string]bool{}
	for _, s := range first {
		direct = append(direct, formatSite(name, s, true, ""))
		decl := enclosingDeclaration(s.lines, s.at)
		outer := declarationName(s.lines, decl)
		// Two sites inside one declaration share its callers, so they are followed once. A wrapper of
		// the same name would only find the first level again. A method is called through objects whose
		// methods share its name, so only a top-level declaration is followed.
		if outer == "" || outer == name || followed[s.file+"\x00"+outer] || len(second) == sitesPerName ||
			columns(indentOf(s.lines, decl)) > 0 {
			continue
		}
		followed[s.file+"\x00"+outer] = true
		span := declarationSpan(s.lines, decl, false)
		inside := func(file string, line int) bool {
			return file == path || file == s.file && len(span) > 0 && line >= span[0] && line <= span[len(span)-1]
		}
		for _, t := range findSites(top, outer, inside) {
			if len(second) == sitesPerName {
				break
			}
			second = append(second, t)
			further = append(further, formatSite(outer, t, false, fmt.Sprintf(" (which runs %s at %s line %d)", name, s.file, s.at)))
		}
	}
	return direct, further
}

// findSites is up to sitesPerName places that use name, outside the lines skip names. A unit test is
// no caller a reader learns a purpose from, an import names without calling, and a name inside a
// string is a word, not a use. A call ranks ahead of a reference.
func findSites(top, name string, skip func(file string, line int) bool) []site {
	// -z puts a NUL after the path and the line number, so a path holding a colon stays whole.
	out, err := gitOut(top, "grep", "--untracked", "-z", "-n", "-w", "-F", "-I", "-e", name, "--", ".")
	if err != nil {
		return nil
	}
	type hit struct {
		file string
		line int
		call bool
	}
	word := regexp.MustCompile(`(?:^|[^\w$])` + regexp.QuoteMeta(name) + `(?:[^\w$]|$)`)
	call := regexp.MustCompile(`(?:^|[^\w$])` + regexp.QuoteMeta(name) + `\s*(?:<[^>]*>)?\(`)
	var hits []hit
	for _, row := range strings.Split(strings.TrimSpace(out), "\n") {
		fields := strings.SplitN(row, "\x00", 3)
		if len(fields) != 3 {
			continue
		}
		file, text := fields[0], strings.TrimSuffix(fields[2], "\r")
		n, err := strconv.Atoi(fields[1])
		if err != nil || skip(file, n) || !sourceExtensions[filepath.Ext(file)] || reUnitTest.MatchString(file) ||
			reImport.MatchString(text) || isComment(strings.TrimSpace(text)) {
			continue
		}
		code := withoutStrings(text, false)
		if !word.MatchString(code) {
			continue
		}
		hits = append(hits, hit{file, n, call.MatchString(code)})
	}
	sort.SliceStable(hits, func(i, j int) bool {
		if hits[i].call != hits[j].call {
			return hits[i].call
		}
		return hits[i].file < hits[j].file
	})
	var sites []site
	for _, h := range hits {
		if len(sites) == sitesPerName {
			break
		}
		body, err := os.ReadFile(filepath.Join(top, h.file))
		if err != nil {
			continue
		}
		lines := strings.Split(strings.TrimSuffix(strings.ReplaceAll(string(body), "\r\n", "\n"), "\n"), "\n")
		if h.line > len(lines) || inImport(lines, h.line) || declarationName(lines, h.line) == name {
			continue
		}
		sites = append(sites, site{h.file, lines, h.line})
	}
	return sites
}

// formatSite sets out one site: where it is, the comment on the declaration around it, and either
// that declaration's whole body, where whole is set and the body is short, or the lines around it.
// A comment block that ends just above the shown lines is shown with them, so no comment is cut off
// at the window's edge.
func formatSite(name string, s site, whole bool, note string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "\n%s is used in %s at line %d%s", name, s.file, s.at, note)
	from, to := max(1, s.at-linesAbove), min(len(s.lines), s.at+linesBelow)
	body := false
	if decl := enclosingDeclaration(s.lines, s.at); decl > 0 {
		fmt.Fprintf(&b, ", inside line %d: %s\n", decl, strings.TrimSpace(s.lines[decl-1]))
		if comment := commentAbove(s.lines, decl); len(comment) > 0 {
			b.WriteString("  whose comment reads:\n")
			for _, line := range comment {
				b.WriteString("    " + strings.TrimSpace(line) + "\n")
			}
		}
		if span := declarationSpan(s.lines, decl, false); whole && len(span) > 0 && len(span) <= maxBodyLines {
			from, to, body = span[0], span[len(span)-1], true
		}
	} else {
		b.WriteString("\n")
	}
	// A whole body's own comment is already shown above it.
	for edge := 0; !body && edge < maxEdgeLines && from > 1 && isComment(strings.TrimSpace(s.lines[from-2])); edge++ {
		from--
	}
	for n := from; n <= to; n++ {
		fmt.Fprintf(&b, "  %4d  %s\n", n, s.lines[n-1])
	}
	return b.String()
}

// reCallChain is a statement that calls through a chain, as `list().forEach(cell => {`, which the
// member pattern would read as a method.
var reCallChain = regexp.MustCompile(`^\s*[A-Za-z_$][\w$.]*\([^)]*\)\s*\.`)

// encloses says the line declares something a reader would name as what a site runs inside: a
// function, a method, a class or a table, never a call chain or a local holding a plain value.
func encloses(line string) bool {
	if !reDeclaration.MatchString(line) || notDeclaration.MatchString(line) || reCallChain.MatchString(line) {
		return false
	}
	if reLocal.MatchString(line) && !strings.Contains(line, "=>") && !strings.Contains(line, "function") {
		return false
	}
	return true
}

// reDeclName is the name a declaration line declares: after its keyword, or a member's own name.
var reDeclName = regexp.MustCompile(`(?:^|\s)(?:function\*?|class|const\s+enum|const|let|var|type|interface|enum|func|def|fn)` +
	`\s+(?:\([^)]*\)\s*)?([A-Za-z_$][\w$]*)|^\s*(?:(?:public|private|protected|static|readonly|async|get|set|override)\s+)*` +
	`([A-Za-z_$][\w$]*)\s*[(<]`)

// declarationName is the name the declaration on the line declares, or empty where the line holds
// no declaration.
func declarationName(lines []string, line int) string {
	if line < 1 || line > len(lines) || !encloses(lines[line-1]) {
		return ""
	}
	m := reDeclName.FindStringSubmatch(lines[line-1])
	if m == nil {
		return ""
	}
	if m[1] != "" {
		return m[1]
	}
	return m[2]
}

// enclosingDeclaration is the nearest declaration above the line that sits at a shallower indent, or
// 0 where the line is at the top level.
func enclosingDeclaration(lines []string, at int) int {
	indent := columns(indentOf(lines, at))
	for n := at - 1; n >= 1; n-- {
		line := lines[n-1]
		if strings.TrimSpace(line) == "" || columns(indentOf(lines, n)) >= indent {
			continue
		}
		if encloses(line) {
			return n
		}
		// A shallower line that is no declaration, such as an if, encloses the site too, so only a line
		// shallower still can be the declaration around it.
		indent = columns(indentOf(lines, n))
	}
	return 0
}

// commentAbove is the comment block that ends just above the declaration, past its decorators.
func commentAbove(lines []string, decl int) []string {
	end := aboveDecorators(lines, decl) - 1
	for _, bl := range commentBlocks(lines[:end]) {
		if bl.Line+bl.Span-1 == end {
			return lines[bl.Line-1 : end]
		}
	}
	return nil
}

// inImport says the line is a name inside a multi-line import or re-export list, which names the
// declaration without calling it.
func inImport(lines []string, at int) bool {
	for n := at - 1; n >= 1; n-- {
		t := strings.TrimSpace(lines[n-1])
		switch {
		case strings.HasPrefix(t, "import ") || strings.HasPrefix(t, "import{") || strings.HasPrefix(t, "export {"):
			// Only a list left open on its own line runs on to the lines below it.
			return strings.HasSuffix(t, "{")
		case t == "" || strings.HasSuffix(t, ";") || strings.Contains(t, "}") || strings.HasSuffix(t, "{"):
			return false
		}
	}
	return false
}
