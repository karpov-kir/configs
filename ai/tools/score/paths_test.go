package score

// Where the two configs are found, and the two results a list that never fully arrived must not
// produce. Both turn on inputs no fixture string can carry: an argv0 that names no directory, and
// a reader that dies partway.

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// --- where the config is found -------------------------------------------------------------------------

func TestConfigPathsAreDerivedFromTheInvokedPath(t *testing.T) {
	env := ConfigPaths("/somewhere/ai/kk-flavor/scripts/score.sh", func(key string) (string, bool) {
		if key == "XDG_CONFIG_HOME" {
			return "/cfg", true
		}
		return "", false
	})
	if want := filepath.Clean("/somewhere/ai/kk-flavor/thresholds.conf"); filepath.Clean(env.ConfigPath) != want {
		t.Errorf("tracked config resolved to %q, wanted %q", env.ConfigPath, want)
	}
	if want := "/cfg/kk-flavor/thresholds.conf"; env.OverridePath != want {
		t.Errorf("override resolved to %q, wanted %q", env.OverridePath, want)
	}
}

// The override lives outside `~/.kk-flavor`, which is a symlink into the repository.
func TestTheOverrideFallsBackToHomeConfig(t *testing.T) {
	env := ConfigPaths("/x/ai/kk-flavor/scripts/score.sh", func(key string) (string, bool) {
		if key == "HOME" {
			return "/home/someone", true
		}
		return "", false
	})
	if want := "/home/someone/.config/kk-flavor/thresholds.conf"; env.OverridePath != want {
		t.Errorf("override resolved to %q, wanted %q", env.OverridePath, want)
	}
}

// A config home that is not absolute resolves against whatever directory the process stands in, and
// this tool is run from inside the tree under review — so a checkout shipping `cfg/kk-flavor/
// thresholds.conf` would choose the bar its own change set is cut against. The XDG spec says to ignore
// a relative value, and `ai/tools/eco-report/root.go` holds the same rule.
func TestARelativeConfigHomeIsNotAnOverride(t *testing.T) {
	env := ConfigPaths("/x/ai/kk-flavor/scripts/score.sh", func(key string) (string, bool) {
		switch key {
		case "XDG_CONFIG_HOME":
			return "cfg", true
		case "HOME":
			return "/home/someone", true
		}
		return "", false
	})
	if want := "/home/someone/.config/kk-flavor/thresholds.conf"; env.OverridePath != want {
		t.Errorf("override resolved to %q, wanted the home fallback %q", env.OverridePath, want)
	}
}

// The same rule one level down: a relative HOME cannot stand in for the absolute one either.
func TestARelativeHomeIsNoOverrideEither(t *testing.T) {
	env := ConfigPaths("/x/ai/kk-flavor/scripts/score.sh", func(key string) (string, bool) {
		if key == "HOME" {
			return "home/someone", true
		}
		return "", false
	})
	if env.OverridePath != "" {
		t.Errorf("override resolved to %q with a relative HOME, wanted empty", env.OverridePath)
	}
}

// An empty path must never be probed as if it were a file.
func TestNoHomeMeansNoOverridePath(t *testing.T) {
	env := ConfigPaths("/x/ai/kk-flavor/scripts/score.sh", func(string) (string, bool) { return "", false })
	if env.OverridePath != "" {
		t.Errorf("override resolved to %q with no HOME, wanted empty", env.OverridePath)
	}
}

// A pipe that dies partway is not an end of list. Swallowed, a list that stopped after two items
// prints `1 kept, 1 cut` at exit 0 — the exact shape a whole scored list takes, over a list nobody
// has all of. Only a reader that fails can drive this: nothing a string can hold reaches the check,
// which is why the guard sat unobserved.
type stoppingReader struct {
	sent string
	gone bool
}

func (s *stoppingReader) Read(into []byte) (int, error) {
	if s.gone {
		return 0, errors.New("the pipe went away")
	}
	s.gone = true
	return copy(into, s.sent), nil
}

func TestAListThatStoppedMidReadIsNotAWholeOne(t *testing.T) {
	r := newRun(t, trackedConfig, nil)
	r.doReading(&stoppingReader{sent: "3\tfirst\n9\tsecond\n"}, "cut", "instruction", "what a 10 is")
	r.expectCode(2)
	r.expectOut("stopped mid-read")
	r.expectOut("what arrived is not the whole list")
	// And the two items it did reach must not come back as a finished count.
	r.expectNotOut("1 kept, 1 cut")
}

// The tracked threshold config is derived from argv0, so argv0 has to name a location. Invoked as a
// bare name — this tool found on PATH — `filepath.Dir` answers "." and the config resolves against the
// working directory. That directory is the tree under review, so it would choose the bar its own
// change set is cut against, which is exactly what the XDG rule beside it refuses.
func TestTheTrackedConfigIsNotResolvedAgainstTheWorkingDirectory(t *testing.T) {
	none := func(string) (string, bool) { return "", false }
	for _, row := range []struct {
		name, argv0 string
		located     bool
	}{
		{"a bare name, found on PATH", "score.sh", false},
		{"a bare name with no extension", "score", false},
		{"a trailing slash, which names a directory not a file", "scripts/", false},
		{"an absolute path, which the stub uses", "/opt/skills/kk/scripts/score.sh", true},
		{"a relative path with a directory in it", "./scripts/score.sh", true},
	} {
		t.Run(row.name, func(t *testing.T) {
			got := ConfigPaths(row.argv0, none).ConfigPath
			if row.located && got == "" {
				t.Errorf("ConfigPaths(%q) located nothing, so a legitimate invocation can no longer "+
					"find the tracked config at all", row.argv0)
			}
			if !row.located && got != "" {
				t.Errorf("ConfigPaths(%q) resolved the tracked config to %s. argv0 names no directory, "+
					"so that path came from the working directory — the tree under review picking its "+
					"own quality bar", row.argv0, got)
			}
		})
	}
}

// An unlocatable config refuses by name rather than falling back, and says what to do about it.
func TestAnUnlocatableConfigRefusesRatherThanGuessing(t *testing.T) {
	_, err := readTable("", nil)
	if err == nil {
		t.Fatal("readTable accepted an empty path, so an unlocatable config reads as a usable one")
	}
	if !strings.Contains(err.Error(), "bare name") {
		t.Errorf("the refusal does not say why the config could not be found, so the operator cannot "+
			"act on it: %v", err)
	}
}

// The fixture above says it holds the tracked config's real shape, and nothing made that true. It
// drifted once already: `record-entry` reached ai/kk-flavor/thresholds.conf through a merge and the
// fixture stayed at seven lanes, so every case here ran against a config the repository does not have.
//
// Compared by lane name rather than byte-for-byte: the fixture deliberately drops the file's prose and
// pins one bar for every lane, so the numbers are its own. What it may not do is rule a different SET
// of lanes from the file it claims to mirror.
func TestTheFixtureRulesTheSameLanesAsTheTrackedConfig(t *testing.T) {
	body, err := os.ReadFile(filepath.Join("..", "..", "kk-flavor", "thresholds.conf"))
	if err != nil {
		t.Fatalf("reading the tracked config: %v — this case cannot check a fixture against a file it "+
			"could not open, and must not pass over that", err)
	}
	ruled := laneNames(string(body))
	if len(ruled) == 0 {
		t.Fatal("the tracked config rules no lanes at all, so this case would pass over any fixture")
	}
	fixture := laneNames(trackedConfig)
	if !slices.Equal(ruled, fixture) {
		t.Errorf("the fixture rules %v; ai/kk-flavor/thresholds.conf rules %v. The comment above the "+
			"fixture claims they are the same shape, and every case here is written on that claim.",
			fixture, ruled)
	}
}

// The lane names a threshold table rules, sorted. Reads the same line form readTable does — a name,
// `cut`, `<=`, a number — so a line this counts is a line the tool would rule on.
func laneNames(body string) []string {
	var names []string
	for _, line := range strings.Split(body, "\n") {
		fields := strings.Fields(strings.TrimSpace(line))
		if len(fields) == 4 && fields[1] == "cut" && fields[2] == "<=" {
			names = append(names, fields[0])
		}
	}
	slices.Sort(names)
	return names
}
