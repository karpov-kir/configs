package ecocheck_test

// The properties report.go's rank table needs, none of them visible in the report it produces. These
// read the table, and one of them reads the package's own source; nothing here runs the checker.

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"strconv"
	"strings"
	"testing"

	ecocheck "kk-flavor/tools/eco-check"
)

// A row hidden behind another silently rejoins the class it was written to leave. A rank whose rows
// outnumber its budget spends that budget on floor lines. And a row ranked past the class that holds
// every unnamed kind puts a kind that WAS named below it.
func TestTheRankTableGivesEachKindAClassItCanAfford(t *testing.T) {
	t.Run("no class prefix swallows another", func(t *testing.T) {
		classes := ecocheck.RankTable()
		for i, class := range classes {
			for j, other := range classes {
				if i != j && strings.HasPrefix(other.Prefix, class.Prefix) {
					t.Errorf("%q (rank %d) classes every finding of %q (rank %d), which reaches its own row never",
						class.Prefix, class.Rank, other.Prefix, other.Rank)
				}
			}
		}
	})

	// The floor spends one line per class present. A rank whose classes outnumber its budget leaves
	// the ones past it with no line at all. Each still prints a note claiming it withheld more than
	// the none it showed. `>=` rather than `>`: the class holding every kind no row names sits at
	// this rank too, taking one of its slots without being a row here. Rank 5 is the wide one, and it
	// still leaves lines to spend past its own floor.
	t.Run("and no rank holds more classes than its own budget of lines", func(t *testing.T) {
		perRank := map[int]int{}
		for _, class := range ecocheck.RankTable() {
			perRank[class.Rank]++
		}
		for rank, classes := range perRank {
			if classes >= ecocheck.FindingCap {
				t.Errorf("rank %d holds %d classes against a budget of %d lines, so its floor alone fills it",
					rank, classes, ecocheck.FindingCap)
			}
		}
	})

	// A kind emitted with no row falls to the class report.go ranks at unnamedClassRank, and the file
	// says that class still ranks last. A row past it takes that away in silence: the kind nobody named
	// would then outrank one somebody did.
	t.Run("and no row ranks past the class that holds every kind no row names", func(t *testing.T) {
		for _, class := range ecocheck.RankTable() {
			if class.Rank > ecocheck.UnnamedClassRank {
				t.Errorf("%q ranks %d, past the %d the unnamed class sits at, so a kind with no row outranks it",
					class.Prefix, class.Rank, ecocheck.UnnamedClassRank)
			}
		}
	})
}

// The claim the whole table rests on: every kind this checker emits has a row. That is what makes a
// class one kind, and a class of one kind is what lets its suppression note say "of this class" and
// mean it. A row and its emit site share a constant, so a reworded head stops compiling — but a kind
// added with a constant of its own and no row compiles fine, falls to the class no row names, and
// prints with no count at all.
//
// What this reads is the head each emit site *leads with*, and only where the source decides it: a
// string literal, a constant this package declares, or the run of a Sprintf format ahead of its first
// verb. Matching looser — a fragment from the middle of a Sprintf, say — reports heads nobody emits.
//
// Three shapes therefore stay unchecked rather than guessed at: a head handed in as a parameter
// (direction.go's four bounded classes), one a helper builds (shell.go's two bounded-read wordings),
// and a finding passed to c.add as a variable (scripts.go's `syntax:` lines, subcommands.go's welded
// names). Each has a row today, and what this case says about them is nothing — never a pass.
//
// The one way a site can be covered wrongly rather than not at all: a local shadowing a head constant
// is read as that constant. Nothing in this package does it, and only a type checker would see it.
func TestEveryEmittedHeadHasARankTableRow(t *testing.T) {
	heads := emittedHeads(t)
	// The extractor is this case's instrument, and a broken one finds no head and passes. Well under
	// what the package emits today, so a shape reworded costs a head rather than the whole run; far
	// enough above zero that an extractor matching nothing cannot read as an answer.
	if len(heads) < 25 {
		t.Fatalf("read %d finding heads out of the package — too few to be this run's answer, so the shapes emittedHeads matches have gone stale",
			len(heads))
	}
	for _, head := range heads {
		if isRanked(head.text) {
			continue
		}
		t.Errorf("%s emits %q and no row of report.go's rank table names it. It falls to the class holding every unnamed kind, which prints no suppression count at all",
			head.at, head.text)
	}
}

func isRanked(head string) bool {
	for _, row := range ecocheck.RankTable() {
		if strings.HasPrefix(head, row.Prefix) {
			return true
		}
	}
	return false
}

// One finding head and the source position that emits it.
type emittedHead struct {
	at   string
	text string
}

// The head every c.add and addCitationFinding site in this package leads with, for the sites whose
// head the source decides. A file that will not parse fails the case rather than shortening its
// answer: a partial read here is a green run over code nobody looked at.
func emittedHeads(t *testing.T) []emittedHead {
	t.Helper()
	fileSet := token.NewFileSet()
	var parsed []*ast.File
	for _, name := range packageSourceNames(t) {
		file, err := parser.ParseFile(fileSet, name, nil, 0)
		if err != nil {
			t.Fatalf("this case measured nothing — %s would not parse: %v", name, err)
		}
		parsed = append(parsed, file)
	}
	// Both passes are over the whole package: a head's constant is declared beside its own scan and
	// read from report.go, so neither file alone holds a row and its value.
	consts := stringConsts(parsed)
	var heads []emittedHead
	for _, file := range parsed {
		ast.Inspect(file, func(node ast.Node) bool {
			call, isCall := node.(*ast.CallExpr)
			if !isCall {
				return true
			}
			finding, isEmit := findingArgument(call)
			if !isEmit {
				return true
			}
			if head, decided := leadingHead(finding, consts); decided {
				heads = append(heads, emittedHead{at: fileSet.Position(call.Pos()).String(), text: head})
			}
			return true
		})
	}
	return heads
}

// This package's own non-test sources. The suite runs with the package directory as its working
// directory, and a directory that yields none of them is a case that cannot run at all.
func packageSourceNames(t *testing.T) []string {
	t.Helper()
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("this case measured nothing — the package directory would not open: %v", err)
	}
	var names []string
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		names = append(names, name)
	}
	if len(names) == 0 {
		t.Fatal("this case measured nothing — no non-test .go file beside it")
	}
	return names
}

// Every package-level `const name = "…"`. Ones bound to anything else — a number, another package's
// constant — are left out, so a head reached through one is undecidable rather than wrong.
func stringConsts(files []*ast.File) map[string]string {
	consts := map[string]string{}
	for _, file := range files {
		for _, decl := range file.Decls {
			declared, isGeneric := decl.(*ast.GenDecl)
			if !isGeneric || declared.Tok != token.CONST {
				continue
			}
			for _, spec := range declared.Specs {
				value, isValue := spec.(*ast.ValueSpec)
				if !isValue || len(value.Names) != len(value.Values) {
					continue
				}
				for i, name := range value.Names {
					if text, isString := stringLiteral(value.Values[i]); isString {
						consts[name.Name] = text
					}
				}
			}
		}
	}
	return consts
}

// The argument that becomes a finding at the two calls that raise one: c.add's only argument, and
// addCitationFinding's second. Matched on the method name, never on the receiver's spelling — a
// receiver renamed would otherwise stop this case matching anything, which is the silence it exists
// to end.
func findingArgument(call *ast.CallExpr) (ast.Expr, bool) {
	selected, isSelector := call.Fun.(*ast.SelectorExpr)
	if !isSelector {
		return nil, false
	}
	switch {
	case selected.Sel.Name == "add" && len(call.Args) == 1:
		return call.Args[0], true
	case selected.Sel.Name == "addCitationFinding" && len(call.Args) == 2:
		return call.Args[1], true
	}
	return nil, false
}

// The text a finding expression starts with, or that the source does not decide it. A concatenation
// is its leftmost leaf; a Sprintf is its format up to the first verb, since past that the text is a
// value's rather than the head's — and a format opening on a verb decides nothing at all.
func leadingHead(finding ast.Expr, consts map[string]string) (string, bool) {
	switch node := finding.(type) {
	case *ast.ParenExpr:
		return leadingHead(node.X, consts)
	case *ast.BinaryExpr:
		if node.Op == token.ADD {
			return leadingHead(node.X, consts)
		}
	case *ast.BasicLit:
		return stringLiteral(node)
	case *ast.Ident:
		text, isConst := consts[node.Name]
		return text, isConst
	case *ast.CallExpr:
		if isSprintf(node.Fun) && len(node.Args) > 0 {
			format, decided := leadingHead(node.Args[0], consts)
			if verb := strings.IndexByte(format, '%'); verb >= 0 {
				format = format[:verb]
			}
			return format, decided && format != ""
		}
	}
	return "", false
}

func isSprintf(function ast.Expr) bool {
	selected, isSelector := function.(*ast.SelectorExpr)
	if !isSelector || selected.Sel.Name != "Sprintf" {
		return false
	}
	pkg, isIdent := selected.X.(*ast.Ident)
	return isIdent && pkg.Name == "fmt"
}

func stringLiteral(expr ast.Expr) (string, bool) {
	literal, isLiteral := expr.(*ast.BasicLit)
	if !isLiteral || literal.Kind != token.STRING {
		return "", false
	}
	text, err := strconv.Unquote(literal.Value)
	return text, err == nil
}
