package ecocheck

import (
	"regexp"
	"sort"
	"strconv"
	"strings"

	"kk-flavor/tools/shell"
)

const dispatchNamesFewerFlags = "dispatch names fewer flags than the lane's own: "

// A reference to a lane script and whatever flags follow it. The flags decide which mode of the tool
// runs, and the reference an orchestrator reads is usually not the one the owning skill wrote — so a
// flag dropped in a file about something else leaves every path still resolving, which is why no other
// check here sees it.
// The path prefix is what separates a dispatch from prose about a script: a record row narrating
// "check.sh's skill-dir scan" names no mode and owes no flags, while every site an agent actually runs
// from is written as a path.
var scriptInvocation = regexp.MustCompile(`(?:[A-Za-z0-9_.~-]+/)+([a-z0-9][a-z0-9-]*\.sh)((?:\s+--[a-z][a-z0-9-]*)*)`)

// scanInvocationSpelling holds every dispatch of a lane's script to the spelling that lane's own skill
// uses. The owning skill is the authority on how its script runs; a caller naming the same script with
// fewer flags sends its reader to another mode of the same tool, and the report that comes back answers
// a different question without saying so.
func (c *checker) scanInvocationSpelling() {
	owned := map[string]map[string]bool{}
	type dispatch struct {
		script, file string
		line         int
		flags        map[string]bool
	}
	var dispatches []dispatch

	for file, lines := range c.filesWithLines(c.root.Named(), "*.md") {
		caller := owningSkill(file)
		for _, hit := range grepNumbered(lines, scriptInvocation) {
			parts := scriptInvocation.FindStringSubmatch(hit.match)
			script, flags := parts[1], flagSet(parts[2])
			if len(c.scriptOwners[script]) != 1 {
				continue
			}
			if owner := owningSkill(c.scriptOwners[script][0]); owner != "" && owner == caller {
				if owned[script] == nil {
					owned[script] = map[string]bool{}
				}
				for flag := range flags {
					owned[script][flag] = true
				}
				continue
			}
			dispatches = append(dispatches, dispatch{script: script, file: shell.Oneline(file), line: hit.line, flags: flags})
		}
	}

	for _, site := range dispatches {
		missing := make([]string, 0, len(owned[site.script]))
		for flag := range owned[site.script] {
			if !site.flags[flag] {
				missing = append(missing, flag)
			}
		}
		if len(missing) == 0 {
			continue
		}
		sort.Strings(missing)
		c.add(dispatchNamesFewerFlags + site.file + ":" + strconv.Itoa(site.line) + " — " +
			shell.Oneline(site.script) + " without " + strings.Join(missing, " ") +
			", which its own skill names; the reader runs another mode and cannot tell")
	}
}

// owningSkill is the skill a path sits under, or "" outside one.
func owningSkill(path string) string {
	parts := strings.Split(path, "/")
	for i, part := range parts {
		if part == "skills" && i+1 < len(parts) {
			return parts[i+1]
		}
	}
	return ""
}

func flagSet(trailing string) map[string]bool {
	flags := map[string]bool{}
	for _, flag := range strings.Fields(trailing) {
		flags[flag] = true
	}
	return flags
}
