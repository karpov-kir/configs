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
// never states. So the call carries, for each exported declaration in the material, a few of its call
// sites found in the repository, each with the comment on the declaration that encloses it and a few
// lines around it. They are context, never material: the call decides nothing in them.

const (
	// sitesPerName bounds the call sites shown for one declaration.
	sitesPerName = 3
	// linesAbove and linesBelow are the context around a call site, eight lines with the site.
	linesAbove = 4
	linesBelow = 3
	// maxCallersBytes bounds the whole section, so a widely called file does not crowd out its own text.
	maxCallersBytes = 12_000
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
	var b strings.Builder
	for _, name := range names {
		for _, site := range callSites(top, path, name) {
			if b.Len()+len(site) > maxCallersBytes {
				break
			}
			b.WriteString(site)
		}
	}
	if b.Len() == 0 {
		return ""
	}
	return "\nCallers of this file's exported declarations, as context only. Decide nothing in them, and " +
		"use them for what a caller relies on:\n" + b.String()
}

// callSites is up to sitesPerName places outside path that use name, each set out with its enclosing
// declaration's comment and the lines around it. A unit test is no caller a reader learns a purpose
// from, and an import names without calling.
func callSites(top, path, name string) []string {
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
		if err != nil || file == path || !sourceExtensions[filepath.Ext(file)] || reUnitTest.MatchString(file) ||
			reImport.MatchString(text) || isComment(strings.TrimSpace(text)) {
			continue
		}
		// A name inside a string literal is a word, not a use.
		code := withoutStrings(text, false)
		if !word.MatchString(code) {
			continue
		}
		hits = append(hits, hit{file, n, call.MatchString(code)})
	}
	// A call shows what a caller relies on better than a reference does, so calls come first.
	sort.SliceStable(hits, func(i, j int) bool {
		if hits[i].call != hits[j].call {
			return hits[i].call
		}
		return hits[i].file < hits[j].file
	})
	var sites []string
	for _, h := range hits {
		if len(sites) == sitesPerName {
			break
		}
		body, err := os.ReadFile(filepath.Join(top, h.file))
		if err != nil {
			continue
		}
		lines := strings.Split(strings.TrimSuffix(strings.ReplaceAll(string(body), "\r\n", "\n"), "\n"), "\n")
		if inImport(lines, h.line) {
			continue
		}
		sites = append(sites, formatSite(name, h.file, lines, h.line))
	}
	return sites
}

// formatSite sets out one call site: where it is, the comment on the declaration around it, and the
// lines around it, numbered.
func formatSite(name, file string, lines []string, at int) string {
	var b strings.Builder
	fmt.Fprintf(&b, "\n%s is used in %s at line %d", name, file, at)
	if decl := enclosingDeclaration(lines, at); decl > 0 {
		fmt.Fprintf(&b, ", inside line %d: %s\n", decl, strings.TrimSpace(lines[decl-1]))
		if comment := commentAbove(lines, decl); len(comment) > 0 {
			b.WriteString("  whose comment reads:\n")
			for _, line := range comment {
				b.WriteString("    " + strings.TrimSpace(line) + "\n")
			}
		}
	} else {
		b.WriteString("\n")
	}
	for n := max(1, at-linesAbove); n <= min(len(lines), at+linesBelow); n++ {
		fmt.Fprintf(&b, "  %4d  %s\n", n, lines[n-1])
	}
	return b.String()
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
		if reDeclaration.MatchString(line) && !notDeclaration.MatchString(line) {
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
