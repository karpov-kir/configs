package ecocheck

import (
	"regexp"
	"strings"

	"kk-flavor/tools/shell"
)

var (
	// The cited file at the head of a `<file> → <Section>`, in its three written forms.
	trailingLinkTarget = regexp.MustCompilePOSIX(`\]\([^()]*\)$`)
	trailingBarePath   = regexp.MustCompilePOSIX(`[A-Za-z0-9._/-]+$`)
	markdownFileTail   = regexp.MustCompilePOSIX(`[A-Za-z0-9]\.md$`)
	sectionCutPoint    = regexp.MustCompilePOSIX(`[():;,.!?"]`)
	// A markdown file named in prose, whichever of the three forms wrote it: the `](…)` and the
	// backticks fall outside the character class, so one pattern reads all three. Anchored at the end,
	// because it is matched against a window that ends on the `.md` being tested.
	//
	// The leading run is optional, and that is load-bearing rather than tidy: required, the shortest
	// name it can match is `aa.md`, so `a.md` matches nothing and every probe past it fails.
	markdownFileTokenAtEnd = regexp.MustCompilePOSIX(`([A-Za-z0-9~][A-Za-z0-9._/~-]*)?[A-Za-z0-9]\.md$`)
)

// What separates a cited file from the section it names.
const sectionArrow = "→"

type citation struct {
	src  string
	line int
	// The cited file. Empty when nothing before the arrow named one and nothing earlier in the block
	// could stand in, which is the citation reportCitation refuses rather than resolves.
	path string
	// What stood immediately before the arrow, for the finding that has no path to name.
	head        string
	section     string
	isDelimited bool
}

func citationsIn(file string, lines []string) []citation {
	var found []citation
	for _, block := range citationBlocks(lines) {
		found = append(found, block.citations(file)...)
	}
	return found
}

// A run of adjacent lines one citation may be spread across, joined by the newline between them, and
// the 1-based number of the first of them.
type citationBlock struct {
	firstLine int
	text      string
}

// The blocks of a file. A citation wraps: a hard line break between the cited file and its section,
// or inside the section name, is what a formatter leaves behind, and reading one line at a time
// cannot see across it — the citation resolves against nothing and the scan goes quiet, which is the
// one outcome a dangling-reference scan must not have.
//
// A blank line ends a block, and so does a fence. A paragraph break is not a wrap, and joining across
// one would let a file named in one paragraph answer an arrow in the next: a citation nobody wrote.
func citationBlocks(lines []string) []citationBlock {
	var blocks []citationBlock
	var current []string
	firstLine := 0
	inFence := false
	closeBlock := func() {
		if len(current) > 0 {
			blocks = append(blocks, citationBlock{firstLine: firstLine, text: strings.Join(current, "\n")})
			current = nil
		}
	}
	for i, line := range lines {
		if shell.IsFenceDelimiter(line) {
			inFence = !inFence
			closeBlock()
			continue
		}
		if inFence || strings.Trim(line, shell.SpaceBytes) == "" {
			closeBlock()
			continue
		}
		if len(current) == 0 {
			firstLine = i + 1
			current = append(current, line)
			continue
		}
		current = append(current, continuationText(line))
	}
	closeBlock()
	return blocks
}

// What a continued line contributes to its block. Its indentation is layout, and a leading `#` is the
// marker of the comment or the heading the citation was written inside — a script cites in a comment,
// so that marker rides onto the continuation line while belonging to neither the path nor the section
// name. citedSection strips a heading marker off a section for the same reason.
func continuationText(line string) string {
	indented := strings.TrimLeft(line, shell.SpaceBytes)
	unmarked := strings.TrimLeft(indented, "#")
	if len(unmarked) == len(indented) {
		return indented
	}
	return strings.TrimLeft(unmarked, shell.SpaceBytes)
}

// The citations in one block, each reported at the line its arrow sits on — where a reader looks for
// it, and the line the unwrapped form has always been reported at.
func (b citationBlock) citations(file string) []citation {
	var found []citation
	segments := strings.Split(b.text, sectionArrow)
	line := b.firstLine
	carried := ""
	for i := 1; i < len(segments); i++ {
		// Both of these are carried along the split rather than recounted from the head of the block:
		// a file choosing to carry thousands of arrows would otherwise cost a pass over the whole
		// block for each one.
		line += strings.Count(segments[i-1], "\n")
		if named := lastMarkdownFileIn(segments[i-1]); named != "" {
			carried = named
		}
		section, isDelimited, isBold := citedSection(segments[i])
		if section == "" {
			continue
		}
		head := headBefore(segments[i-1])
		cited := citation{
			src:         file,
			line:        line,
			path:        head.path,
			head:        head.token,
			section:     section,
			isDelimited: isDelimited,
		}
		if head.path == "" {
			// Nothing immediately before the arrow named a file, and only the mandated `**` form is
			// read as a citation from here on: `intent → **build**` is prose, but so is every other
			// arrow in this tree whose right side is bare or backticked, and admitting those turns
			// every prose arrow near a filename into a finding.
			if !isBold {
				continue
			}
			// The file named earlier in the same block, which is how this tree writes a second
			// citation into a file it has already named: `You run under `x.md` … (→ **Section**)`.
			// Without this the arrow resolves against nothing and the scan says nothing — the silence
			// a dangling-reference scan may not have. Bounded to the block for the reason the block
			// exists: a file named in one paragraph must not answer an arrow in the next.
			cited.path = carried
			// Still nothing, and the head was backticked: that is the uncheckable citation, and
			// reportCitation names it. A bare head with no file in the block is prose that happened to
			// bold its right side, so it stays out.
			if cited.path == "" && !head.isBackticked {
				continue
			}
		}
		found = append(found, cited)
	}
	return found
}

// The two bounds the search below runs under, both because the reviewed tree writes the text. The
// first is the longest path read back as a cited file: past it the window is cut, so a committed path
// longer than any anyone writes yields a shorter name that resolves nowhere rather than a scan
// reading megabytes backwards. The second is how many `.md` occurrences are tried before the answer
// is taken to be none — a paragraph of a million bare `.md` tokens forms no filename at all, and each
// one costs a match against the window it sits in.
const (
	maxNamedPathBytes = 1024
	maxNameProbes     = 64
)

// The last markdown file named anywhere in a run of text, in any of the three written forms.
//
// Searched backwards from the end inside a bounded window, never by collecting every match: the text
// is a whole block, the reviewed tree chooses how long a block is, and one committed paragraph of a
// million `.md` tokens would otherwise be held in memory as a million matched strings.
func lastMarkdownFileIn(text string) string {
	for at, probes := len(text), 0; at > 0 && probes < maxNameProbes; probes++ {
		hit := strings.LastIndex(text[:at], ".md")
		if hit < 0 {
			return ""
		}
		at = hit
		end := hit + len(".md")
		// `a.mdx` ends on no markdown file: the suffix has to be where the name ends.
		if end < len(text) && isPathByte(text[end]) {
			continue
		}
		start := max(hit-maxNamedPathBytes, 0)
		if named := markdownFileTokenAtEnd.FindString(text[start:end]); named != "" {
			return named
		}
	}
	return ""
}

func isPathByte(b byte) bool {
	return isAlnumByte(b) || b == '.' || b == '_' || b == '-' || b == '/' || b == '~'
}

// What stood immediately before an arrow.
type citedHead struct {
	// The cited file: a markdown link, a backticked path, or a bare filename. Empty unless the token
	// is a markdown filename, which is what keeps a prose arrow out.
	path string
	// That token whichever way the test went, so a finding with no path to name has something to
	// quote back.
	token string
	// Whether the token arrived inside backticks. A backticked run that is not a markdown file is
	// written as a reference and read as none, which is the one head shape worth a finding of its own.
	isBackticked bool
}

func headBefore(before string) citedHead {
	before = strings.TrimRight(before, shell.SpaceBytes)
	// The cited file sits immediately before the arrow, so only the line the text ends on can carry
	// it. Cut to that line rather than let the patterns below reach back over a wrap: `$` under
	// MustCompilePOSIX is the end of a *line*, so `[A-Za-z0-9._/-]+$` over several of them answers
	// with the leftmost line-final token — `bash` out of a script's `#!` line, never the cited path.
	if wrap := strings.LastIndexByte(before, '\n'); wrap >= 0 {
		before = before[wrap+1:]
	}
	head := citedHead{}
	switch {
	case trailingLinkTarget.MatchString(before):
		open := strings.LastIndex(before, "](")
		head.token = strings.TrimSuffix(before[open+2:], ")")
	case strings.HasSuffix(before, "`"):
		head.isBackticked = true
		unticked := before[:len(before)-1]
		if tick := strings.LastIndexByte(unticked, '`'); tick >= 0 {
			head.token = unticked[tick+1:]
		}
	default:
		head.token = trailingBarePath.FindString(before)
	}
	head.path, _, _ = strings.Cut(head.token, "#")
	if !markdownFileTail.MatchString(head.path) {
		head.path = ""
	}
	return head
}

// Whether the name arrived inside `**` or backticks is the whole decision here: read exactly, or
// guessed at by the fallback. ecosystem.md → **Conventions a new file joins** requires the
// delimited form for that reason.
//
// The `**` half is told from the backticked half as well, because it is the form that file mandates
// and the only one a head naming no file is read through: a bare or backticked right side after a
// prose arrow is how this tree writes prose, and reading those as citations would report the tree
// against itself.
func citedSection(after string) (section string, isDelimited, isBold bool) {
	after = strings.TrimLeft(after, shell.SpaceBytes)
	if rest, ok := strings.CutPrefix(after, "**"); ok {
		if end := strings.Index(rest, "**"); end > 0 {
			section = rest[:end]
			isBold = true
		}
	} else if rest, ok := strings.CutPrefix(after, "`"); ok {
		if end := strings.IndexByte(rest, '`'); end > 0 {
			section = rest[:end]
		}
	}
	isDelimited = section != ""
	if section == "" {
		section = after
		// An undelimited name carries no end marker, so the line it starts on is the only bound it
		// has. The delimited forms above carry their own and may wrap; this one may not.
		if newline := strings.IndexByte(section, '\n'); newline >= 0 {
			section = section[:newline]
		}
		if cut := sectionCutPoint.FindStringIndex(section); cut != nil {
			section = section[:cut[0]]
		}
		if dash := strings.Index(section, "—"); dash > 0 {
			section = section[:dash]
		}
	}
	section = strings.NewReplacer("`", "", "*", "").Replace(section)
	section = headingMarker.ReplaceAllString(section, "")
	return strings.Trim(section, shell.SpaceBytes), isDelimited, isBold
}
