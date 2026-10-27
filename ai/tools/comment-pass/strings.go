package commentpass

import (
	"path/filepath"
	"regexp"
	"strings"
	"unicode/utf8"
)

// codeView is the file with every string literal blanked, its quotes included, and its comments as
// they are. A string may span lines: a template literal, a raw or triple-quoted string, a Rust string,
// a heredoc. The comment finder reads this view, so a line of a string never reads as a comment. A
// string still open at the file's end was misread from the line it opened on, which is returned, and
// the view blanks every line after it, so none of them reads as a comment.
func codeView(lines []string, path string) (view []string, unclosed int) {
	s := scanner{family: familyOf(path), hash: hashCommented(path)}
	out := make([]string, len(lines))
	for i, line := range lines {
		wasOpen := s.isOpen()
		out[i] = s.line(line)
		if !wasOpen && s.isOpen() {
			unclosed = i + 1
		}
	}
	if s.isOpen() {
		for i := unclosed; i < len(out); i++ {
			out[i] = ""
		}
		return out, unclosed
	}
	return out, 0
}

// firstOf is the view alone, for a reader that needs no word on a misread.
func firstOf(view []string, _ int) []string { return view }

// family is a string syntax that files of some extensions write.
type family string

const (
	familyNone  family = ""
	familyJs    family = "js"
	familyJvm   family = "jvm"
	familyGo    family = "go"
	familyPy    family = "py"
	familySh    family = "sh"
	familyRs    family = "rs"
	familyKt    family = "kt"
	familySwift family = "swift"
)

// familyOf names the string syntax a file's extension writes.
func familyOf(path string) family {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".ts", ".tsx", ".js", ".jsx", ".mjs", ".cjs":
		return familyJs
	case ".java":
		return familyJvm
	case ".go":
		return familyGo
	case ".py":
		return familyPy
	case ".sh":
		return familySh
	case ".rs":
		return familyRs
	case ".kt":
		return familyKt
	case ".swift":
		return familySwift
	}
	return familyNone
}

// quote is a string literal's syntax: what opens it, what closes it, whether a backslash escapes the
// next character, and whether it may run past its line's end.
type quote struct {
	open, close string
	hasEscapes  bool
	isMultiline bool
}

// quotes are each family's string literals, the longer opener first.
var quotes = map[family][]quote{
	familyJs:    {{"`", "`", true, true}, {`"`, `"`, true, false}, {"'", "'", true, false}},
	familyGo:    {{"`", "`", false, true}, {`"`, `"`, true, false}, {"'", "'", true, false}},
	familyPy:    {{`"""`, `"""`, true, true}, {"'''", "'''", true, true}, {`"`, `"`, true, false}, {"'", "'", true, false}},
	familyJvm:   {{`"""`, `"""`, true, true}, {`"`, `"`, true, false}, {"'", "'", true, false}},
	familyKt:    {{`"""`, `"""`, false, true}, {`"`, `"`, true, false}, {"'", "'", true, false}},
	familySwift: {{`"""`, `"""`, true, true}, {`"`, `"`, true, false}},
	familySh:    {{`"`, `"`, true, false}, {"'", "'", false, false}},
	familyRs:    {{`"`, `"`, true, true}},
}

// reRawRust opens a Rust raw string, `r"`, `r#"` or `br##"`, whose close repeats its hashes.
var reRawRust = regexp.MustCompile(`^b?r(#*)"`)

// reRawSwift opens a Swift raw string, `#"` or `##"""`, whose close repeats its hashes.
var reRawSwift = regexp.MustCompile(`^(#+)("""|")`)

// reHeredoc opens a shell heredoc, whose lines run to the one holding only its delimiter, quoted or
// bare.
var reHeredoc = regexp.MustCompile(`(?:^|[^<])<<-?\s*(?:'([^']+)'|"([^"]+)"|\\?([^\s;|&<>()'"]+))`)

// reRegexBefore is the code a JavaScript regex literal follows, where a `/` cannot divide.
var reRegexBefore = regexp.MustCompile(`(?:^|[(,=:\[!&|?{};]|=>|\b(?:return|typeof|case|yield|await|throw|delete|void|in|of|else|do))\s*$`)

// arithmetic is each span of the line that shell arithmetic holds, where `<<` is a shift: a `((`
// to the `))` its parentheses close on, or a whole `let` line.
func arithmetic(code string) [][2]int {
	if strings.HasPrefix(strings.TrimSpace(code), "let ") {
		return [][2]int{{0, len(code)}}
	}
	var spans [][2]int
	for at := 0; at+1 < len(code); at++ {
		if code[at] != '(' || code[at+1] != '(' {
			continue
		}
		depth, end := 0, len(code)
		for i := at; i < len(code); i++ {
			if code[i] == '(' {
				depth++
			} else if code[i] == ')' {
				if depth--; depth == 0 {
					end = i + 1
					break
				}
			}
		}
		spans = append(spans, [2]int{at, end})
		at = end
	}
	return spans
}

// scanner carries what is open at a line's end into the next line.
type scanner struct {
	family  family
	hash    bool
	open    *quote
	inStar  bool
	heredoc string
}

// isOpen says a string or a heredoc runs on past the line last read.
func (s *scanner) isOpen() bool {
	return s.open != nil || s.heredoc != ""
}

// line is one line of the view: the code and comments it holds, with its strings blanked.
func (s *scanner) line(line string) string {
	if s.heredoc != "" {
		if strings.TrimSpace(line) == s.heredoc {
			s.heredoc = ""
			return line
		}
		return ""
	}
	r := []rune(line)
	comment := len(r)
	for i := 0; i < len(r); {
		switch {
		case s.inStar:
			if hasAt(r, i, "*/") {
				s.inStar = false
				i += 2
				continue
			}
			i++
		case s.open != nil:
			switch {
			case s.open.hasEscapes && r[i] == '\\':
				blank(r, i, i+2)
				i += 2
			case hasAt(r, i, s.open.close):
				blank(r, i, i+len(s.open.close))
				i += len(s.open.close)
				s.open = nil
			default:
				blank(r, i, i+1)
				i++
			}
		case !s.hash && hasAt(r, i, "//"), s.hash && r[i] == '#' && (s.family != familySh || i == 0 || r[i-1] == ' ' || r[i-1] == '\t'):
			comment = i
			i = len(r)
		case !s.hash && hasAt(r, i, "/*"):
			s.inStar = true
			i += 2
		default:
			i = s.openAt(r, i)
		}
	}
	if s.open != nil && !s.open.isMultiline {
		s.open = nil
	}
	if s.family == familySh {
		s.heredoc = heredocOpened(string([]rune(line)[:comment]), r)
	}
	return string(r)
}

// blank turns r[from:to] to spaces, stopping at r's end.
func blank(r []rune, from, to int) {
	for ; from < to && from < len(r); from++ {
		r[from] = ' '
	}
}

// heredocOpened is the delimiter of the heredoc the line opens, or empty where it opens none. A `<<`
// inside a string, which the view has blanked, or inside arithmetic, is no heredoc.
func heredocOpened(code string, view []rune) string {
	spans := arithmetic(code)
	for _, m := range reHeredoc.FindAllStringSubmatchIndex(code, -1) {
		at := strings.Index(code[m[0]:], "<<") + m[0]
		inside := false
		for _, a := range spans {
			inside = inside || at >= a[0] && at < a[1]
		}
		if r := utf8.RuneCountInString(code[:at]); inside || r >= len(view) || view[r] != '<' {
			continue
		}
		for g := 2; g <= 6; g += 2 {
			if m[g] >= 0 {
				return code[m[g]:m[g+1]]
			}
		}
	}
	return ""
}

// openAt opens the string literal at r[i], blanking its opener, and returns where the scan goes on.
func (s *scanner) openAt(r []rune, i int) int {
	switch s.family {
	case familyJs:
		if r[i] == '/' && reRegexBefore.MatchString(string(r[:i])) {
			return regexEnd(r, i)
		}
	case familySwift:
		if m := reRawSwift.FindStringSubmatch(string(r[i:])); m != nil {
			return s.openQuote(r, i, quote{open: m[0], close: m[2] + m[1], isMultiline: m[2] == `"""`})
		}
	case familyRs:
		if i == 0 || !isIdentifierRune(r[i-1]) {
			if m := reRawRust.FindStringSubmatch(string(r[i:])); m != nil {
				return s.openQuote(r, i, quote{open: m[0], close: `"` + m[1], isMultiline: true})
			}
		}
		if r[i] == '\'' {
			return charEnd(r, i)
		}
	}
	for _, q := range quotes[s.family] {
		if hasAt(r, i, q.open) {
			return s.openQuote(r, i, q)
		}
	}
	return i + 1
}

// openQuote opens q at r[i], blanking its opener, and returns where the scan goes on.
func (s *scanner) openQuote(r []rune, i int, q quote) int {
	s.open = &q
	end := i + len([]rune(q.open))
	blank(r, i, end)
	return end
}

// regexEnd blanks the JavaScript regex literal opening on the `/` at r[i] and returns where the scan
// goes on. A regex literal closes on its line, past its classes. A `/` that finds no close, or whose
// close opens a comment, divides.
func regexEnd(r []rune, i int) int {
	isClass := false
	for end := i + 1; end < len(r); end++ {
		switch {
		case r[end] == '\\':
			end++
		case r[end] == '[':
			isClass = true
		case r[end] == ']':
			isClass = false
		case r[end] == '/' && !isClass:
			if end+1 < len(r) && (r[end+1] == '/' || r[end+1] == '*') {
				return i + 1
			}
			blank(r, i, end+1)
			return end + 1
		}
	}
	return i + 1
}

// charEnd blanks the Rust character literal opening on the `'` at r[i] and returns where the scan goes
// on. A character literal closes within a few characters, where a lifetime opens nothing.
func charEnd(r []rune, i int) int {
	switch {
	case i+1 < len(r) && r[i+1] == '\\':
		end := i + 2
		for end < len(r) && r[end] != '\'' {
			end++
		}
		blank(r, i, end+1)
		return end + 1
	case i+2 < len(r) && r[i+2] == '\'':
		blank(r, i, i+3)
		return i + 3
	}
	return i + 1
}

// hasAt says r holds want at i.
func hasAt(r []rune, i int, want string) bool {
	w := []rune(want)
	if i+len(w) > len(r) {
		return false
	}
	for j, c := range w {
		if r[i+j] != c {
			return false
		}
	}
	return true
}

func isIdentifierRune(c rune) bool {
	return c == '_' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9'
}
