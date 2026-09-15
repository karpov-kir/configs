package ecoreport_test

// No case in this package may conclude anything from the clock. A select arm the clock feeds is allowed
// to end the run — a hang in the code under test is a real defect, and its goroutine dump is worth
// paying for — but it may never reach an assertion.
//
// Absence inside a window is not evidence when the window can be taken away. records_test.go's lock case
// gave a competing write one second in which to not land, and read the silence as proof the lock held.
// Under a reproduction of concurrent-gate load it read it that way over a lock deliberately broken to
// exclude nothing, and passed. That direction is silent — a starved run and a correct one print the same
// two green lines — so nobody re-runs it and nobody looks.
//
// The rule is enforced here rather than remembered because the case named the hazard in its own comment
// and stood for as long as it did anyway.

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNoCaseConcludesAnythingFromTheClock(t *testing.T) {
	t.Parallel()
	names, err := filepath.Glob("*_test.go")
	if err != nil {
		t.Fatal(err)
	}
	arms := 0
	for _, name := range names {
		source, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		concluding, found := clockArms(t, name, string(source))
		arms += found
		for _, arm := range concluding {
			t.Errorf("%s reaches an assertion from %s. A clock arm may only end the run with t.Fatal: "+
				"a loaded machine can take the window away, and the pass that follows prints exactly "+
				"like an earned one. Wait for the event that proves the thing happened instead.", name, arm)
		}
	}
	if arms == 0 {
		t.Fatal("no case here bounds a select on the clock at all, so this would pass over the suite in any state")
	}
}

// The scan above driven over text, so the spellings it exists to catch are cases rather than shapes the
// tree merely happens not to hold today.
func TestWhatCountsAsAClockArmThatConcludes(t *testing.T) {
	for _, row := range []struct {
		name       string
		source     string
		concluding []string
		arms       int
	}{
		{"an arm that ends the run", "func x() { select { case <-c: case <-time.After(30 * time.Second): t.Fatal(\"never landed\") } }", nil, 1},
		{"an arm that ends the run with a formatted reason", "func x() { select { case <-c: case <-time.After(notThis): t.Fatalf(\"never landed: %v\", e) } }", nil, 1},
		{"an arm that records a pass", "func x() { select { case <-c: case <-time.After(time.Second): f.record(\"it waited\", true, \"\") } }", []string{"time.After(time.Second)"}, 1},
		{"an arm that quietly carries on", "func x() { select { case <-c: case <-time.After(time.Second): } }", []string{"time.After(time.Second)"}, 1},
		{"an arm that asserts beside ending the run", "func x() { select { case <-c: case <-time.After(time.Second): f.record(\"it waited\", true, \"\"); t.Fatal(\"x\") } }", []string{"time.After(time.Second)"}, 1},
		{"an arm fed by a ticker", "func x() { select { case <-c: case <-time.Tick(time.Second): f.record(\"it waited\", true, \"\") } }", []string{"time.Tick(time.Second)"}, 1},
		{"an arm fed by a timer's channel", "func x() { select { case <-c: case <-time.NewTimer(time.Second).C: f.record(\"it waited\", true, \"\") } }", []string{"time.NewTimer(time.Second)"}, 1},
		{"an arm on a channel a helper hands back", "func x() { select { case <-f.ready(): f.record(\"it waited\", true, \"\") } }", nil, 0},
		{"an arm on the event the write reached the lock", "func x() { select { case <-landed: return false\ndefault: } }", nil, 0},
		{"a clock call bounding no arm", "func x() { <-time.After(time.Second) }", nil, 0},
	} {
		t.Run(row.name, func(t *testing.T) {
			concluding, arms := clockArms(t, "fixture_test.go", "package ecoreport_test\n"+row.source+"\n")
			if arms != row.arms {
				t.Errorf("found %d clock arm(s), want %d", arms, row.arms)
			}
			if strings.Join(concluding, ",") != strings.Join(row.concluding, ",") {
				t.Errorf("concluding = %q, want %q", concluding, row.concluding)
			}
		})
	}
}

// clockArms reports the select arms in `source` that are bounded by the clock and do anything other
// than end the run, and how many clock-bounded arms it found at all — the second so a scan that matched
// nothing can say so rather than read as a clean sweep. The arm is quoted as the author spelled it, so
// what a failure prints is what the reader will search the file for.
func clockArms(t *testing.T, name, source string) ([]string, int) {
	t.Helper()
	fileSet := token.NewFileSet()
	parsed, err := parser.ParseFile(fileSet, name, source, 0)
	if err != nil {
		t.Fatalf("parsing %s: %v", name, err)
	}
	var concluding []string
	found := 0
	ast.Inspect(parsed, func(node ast.Node) bool {
		clause, isClause := node.(*ast.CommClause)
		if !isClause {
			return true
		}
		clock, bounded := clockCall(clause.Comm)
		if !bounded {
			return true
		}
		found++
		if !endsTheRun(clause.Body) {
			concluding = append(concluding,
				source[fileSet.Position(clock.Pos()).Offset:fileSet.Position(clock.End()).Offset])
		}
		return true
	})
	return concluding, found
}

// The call that puts a clock on a select arm: any of time's, wherever in the arm it is spelled, so
// `case <-time.After(d):`, `case at := <-time.After(d):` and `case <-time.NewTimer(d).C:` are one shape
// rather than three the rule has to list. A `default:` clause carries no statement at all.
func clockCall(comm ast.Stmt) (*ast.CallExpr, bool) {
	if comm == nil {
		return nil, false
	}
	var clock *ast.CallExpr
	ast.Inspect(comm, func(node ast.Node) bool {
		call, isCall := node.(*ast.CallExpr)
		if !isCall {
			return true
		}
		selector, isSelector := call.Fun.(*ast.SelectorExpr)
		if !isSelector {
			return true
		}
		if pkg, isName := selector.X.(*ast.Ident); isName && pkg.Name == "time" {
			clock = call
			return false
		}
		return true
	})
	return clock, clock != nil
}

// A clock arm may do one thing: end the run. One statement, and that statement a t.Fatal — anything
// beside it is a verdict reached because a window expired, and an empty body is the quietest one of all.
func endsTheRun(body []ast.Stmt) bool {
	if len(body) != 1 {
		return false
	}
	statement, isExpr := body[0].(*ast.ExprStmt)
	if !isExpr {
		return false
	}
	call, isCall := statement.X.(*ast.CallExpr)
	if !isCall {
		return false
	}
	selector, isSelector := call.Fun.(*ast.SelectorExpr)
	return isSelector && (selector.Sel.Name == "Fatal" || selector.Sel.Name == "Fatalf")
}
