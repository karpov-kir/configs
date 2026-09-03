// ai/gate.sh keys its Go unit on a handful of inputs that live outside this module, because Go's own
// test cache is content-keyed over the module and cannot see them. Each one is a shell variable, and
// wiring a new one takes two edits in two places: the variable has to reach the `gotest` unit's input
// list, so a change to it invalidates the cached verdict, and it has to reach run_gotest's forcing
// decision, so the package that reads it is re-run with -count=1.
//
// Do only the first and the gate re-runs the unit but answers out of Go's cache — `ok (cached)` over a
// tree the input just moved in. Do only the second and it forces a package on a verdict it was never
// keyed to. Both read as a pass, which is why neither shows up as a failure anyone chases.
//
// This case holds the two lists to each other. It is deliberately structural: what it proves is that
// no external input is wired half way, not what the forcing does at runtime. Adding the fifth of these
// variables is what earned it — the comment on `lines_under` in ai/gate.sh already records a bug of
// the neighbouring shape, where an input set that matched nothing read as "always changed".
package tools_test

import (
	"os"
	"slices"
	"strings"
	"testing"
)

func TestEveryExternalGateInputIsWiredBothWays(t *testing.T) {
	body, err := os.ReadFile(gateScript)
	if err != nil {
		t.Fatalf("reading %s: %v", gateScript, err)
	}
	script := string(body)

	declared := externalInputVars(script)
	if len(declared) == 0 {
		t.Fatalf("%s declares no ext_* variable at the start of a line, so this case has nothing to "+
			"check and would pass over any wiring at all. Either the naming convention changed and this "+
			"case needs to follow it, or the external inputs are gone and it should be retired.",
			gateScript)
	}

	unitLine, ok := gotestUnitLine(script)
	if !ok {
		t.Fatalf("%s has no `unit gotest check` line, so the input list this case reads does not exist. "+
			"The unit was renamed or removed; do not leave this case green over nothing.", gateScript)
	}

	// The input list is one quoted, space-separated field, so dropping the quotes leaves each reference
	// as its own token. Token equality rather than strings.Contains: `$ext_gate` is a substring of a
	// later `$ext_gate_units`, so a substring match lets the second vouch for the first and reports
	// green over exactly the half-wired input this case exists to catch.
	keyedRefs := strings.Fields(strings.ReplaceAll(unitLine, `"`, " "))

	for _, name := range declared {
		keyed := slices.Contains(keyedRefs, "$"+name)
		forced := strings.Contains(script, `changed_since_green "$`+name+`"`)
		switch {
		case keyed && forced:
		case !keyed && !forced:
			t.Errorf("%s declares %s and then uses it for neither: it is not in the gotest unit's input "+
				"list and no branch forces on it. Wire it both ways or delete it — a declared input "+
				"nothing reads is a gate narrower than it looks.", gateScript, name)
		case !keyed:
			t.Errorf("%s forces packages on %s but leaves it out of the gotest unit's input list, so the "+
				"unit is not keyed on a file it re-runs for. The cached verdict survives a change to it, "+
				"and the forcing only happens on runs something else already triggered.", gateScript, name)
		default:
			t.Errorf("%s keys the gotest unit on %s but never calls changed_since_green on it, so a change "+
				"there re-runs the unit and Go then answers out of its own cache — which cannot see a file "+
				"outside this module. The suite reads `ok (cached)` over the tree that just moved.",
				gateScript, name)
		}
	}
}

// The names ai/gate.sh assigns at the start of a line and prefixes `ext_`, which is that file's
// convention for an input living outside this Go module. Anchored at column zero, so a mention inside
// a comment or a nested scope is not read as a declaration.
func externalInputVars(script string) []string {
	var names []string
	for _, line := range strings.Split(script, "\n") {
		if !strings.HasPrefix(line, "ext_") {
			continue
		}
		name, _, found := strings.Cut(line, "=")
		if !found || strings.ContainsAny(name, " \t") {
			continue
		}
		names = append(names, name)
	}
	return names
}

// The line declaring the `gotest` unit, whose third field is the input list every ext_ variable has to
// reach. Line-based for the reason gateStep is: this module carries no shell parser, and the file it
// reads is not one it can import.
func gotestUnitLine(script string) (string, bool) {
	for _, line := range strings.Split(script, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "unit gotest check") {
			return trimmed, true
		}
	}
	return "", false
}
