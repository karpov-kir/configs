package installer_test

// The bound itself. Every other case in this package is written on the assumption that a run cannot
// write outside the tree it was given, and that assumption needs cases of its own.

// The control is among them. A guard that refuses every write would satisfy the two refusal cases
// here. A guard that refuses no write would fail them both, while looking exactly as quiet.

// The root here is a subdirectory of the case's own temp directory, and the paths a case aims
// outside it are still inside that temp directory. A broken guard then writes somewhere harmless and
// the assertion catches it. A case aimed at a real path outside would make the negative control the
// incident.

import (
	"os"
	"strings"
	"testing"

	"configs/ai/tools/installer"
	"configs/ai/tools/installertest"
)

// A run bounded to base/sandbox, with a source to link and a place outside the bound to aim at.
func newBoundedRun(t *testing.T) (base string, run *installer.Run, out *strings.Builder) {
	t.Helper()
	base = installertest.Physical(t, t.TempDir())
	for _, dir := range []string{base + "/sandbox", base + "/outside", base + "/checkout"} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("the fixture could not create %s: %v", dir, err)
		}
	}
	if err := os.WriteFile(base+"/checkout/"+scriptName, []byte("#!/usr/bin/env bash\n"), 0o644); err != nil {
		t.Fatalf("the fixture could not write the installer: %v", err)
	}
	out = &strings.Builder{}
	run = installer.NewRun(installer.RunOptions{
		Repo:       base + "/checkout",
		ScriptName: scriptName,
		Label:      label,
		Out:        out,
		WriteRoot:  base + "/sandbox",
	})
	return base, run, out
}

func TestTheBoundRefusesAWriteOutsideTheTreeItWasGiven(t *testing.T) {
	t.Parallel()
	t.Run("a target whose parent is plainly outside the root is refused", func(t *testing.T) {
		base, run, _ := newBoundedRun(t)
		run.AddConfig(base+"/checkout/"+scriptName, base+"/outside/.zshrc")

		run.Mount()

		if len(run.Breaches()) != 1 {
			t.Fatalf("the bound recorded %d breach(es), wanted 1: %v", len(run.Breaches()), run.Breaches())
		}
		if len(run.Refusals()) != 1 {
			t.Errorf("a breach was not also refused, so the run could still report ok: %v", run.Refusals())
		}
		if _, err := os.Lstat(base + "/outside/.zshrc"); err == nil {
			t.Errorf("the write landed outside the root, which is the whole of what this guard prevents")
		}
	})

	// The shape the incident took: a live symlink inside the tree pointing out of it. A textual
	// comparison of the path passes this and the write lands in whatever the link names — which was a
	// real config file in the working tree.
	t.Run("and so is one reached through a symlink out of the root", func(t *testing.T) {
		base, run, _ := newBoundedRun(t)
		if err := os.Symlink(base+"/outside", base+"/sandbox/escape"); err != nil {
			t.Fatalf("the fixture could not build the escape link: %v", err)
		}
		run.AddConfig(base+"/checkout/"+scriptName, base+"/sandbox/escape/.zshrc")

		run.Mount()

		if len(run.Breaches()) != 1 {
			t.Fatalf("the bound recorded %d breach(es), wanted 1: %v", len(run.Breaches()), run.Breaches())
		}
		if _, err := os.Lstat(base + "/outside/.zshrc"); err == nil {
			t.Errorf("the write followed the link out of the root")
		}
	})

	// The control, and the half that carries the rest. A guard that refused every write would pass
	// both cases here, and every other case in this package would then assert against a run that
	// writes no file at all.
	t.Run("while a write inside the root goes through", func(t *testing.T) {
		base, run, _ := newBoundedRun(t)
		run.AddConfig(base+"/checkout/"+scriptName, base+"/sandbox/.zshrc")

		run.Mount()

		if breaches := run.Breaches(); len(breaches) != 0 {
			t.Fatalf("a write inside the root was recorded as a breach: %v", breaches)
		}
		if value, err := os.Readlink(base + "/sandbox/.zshrc"); err != nil || value != base+"/checkout/"+scriptName {
			t.Errorf("the link inside the root was not made: %s, %v", value, err)
		}
	})
}

// An unbounded run is what an installer on a real machine is, and no write in it may read as a
// breach. The field that exists for the suites would otherwise change what production does.
func TestAnUnboundedRunRecordsNoBreach(t *testing.T) {
	t.Parallel()
	base := installertest.Physical(t, t.TempDir())
	if err := os.MkdirAll(base+"/checkout", 0o755); err != nil {
		t.Fatalf("the fixture could not create the checkout: %v", err)
	}
	if err := os.WriteFile(base+"/checkout/"+scriptName, []byte("#!/usr/bin/env bash\n"), 0o644); err != nil {
		t.Fatalf("the fixture could not write the installer: %v", err)
	}
	run := installer.NewRun(installer.RunOptions{
		Repo:       base + "/checkout",
		ScriptName: scriptName,
		Label:      label,
		Out:        &strings.Builder{},
	})
	run.AddConfig(base+"/checkout/"+scriptName, base+"/anywhere/.zshrc")

	run.Mount()

	if breaches := run.Breaches(); len(breaches) != 0 {
		t.Errorf("an unbounded run recorded %v, which would make every real install look contained", breaches)
	}
	if _, err := os.Readlink(base + "/anywhere/.zshrc"); err != nil {
		t.Errorf("an unbounded run did not write the link it declared: %v", err)
	}
}
