package installer_test

// Which projects this machine installed into. The case that carries the rest is the self-healing
// one. A registry that keeps naming a directory the human deleted makes an uninstall say another
// project still needs the shared bucket. That is false, and it stops a human finishing a removal
// they meant.

import (
	"os"
	"strings"
	"testing"

	"configs/ai/tools/installer"
)

// Two project directories and a run whose registry lives under the case's own tree.
func newRegistryFixture(t *testing.T) (*fixture, *installer.Run) {
	t.Helper()
	f := newBareFixture(t)
	f.MkdirAll(f.base + "/p1")
	f.MkdirAll(f.base + "/p2")
	return f, f.newRun(installer.RunOptions{ConfigHome: f.base + "/cfg"})
}

func registryBody(t *testing.T, run *installer.Run) string {
	t.Helper()
	content, err := os.ReadFile(run.RegistryFile())
	if err != nil {
		return ""
	}
	return string(content)
}

func TestAProjectIsRecordedOnceHoweverOftenItIsInstalled(t *testing.T) {
	t.Parallel()
	f, run := newRegistryFixture(t)

	run.RecordInstall(f.base + "/p1")
	f.ExpectSaid("recorded")
	f.ExpectNoBreach(run.Breaches())

	run.RecordInstall(f.base + "/p1")
	f.ExpectSaid("already recorded")

	run.RecordInstall(f.base + "/p2")

	if got := registryBody(t, run); got != f.base+"/p1\n"+f.base+"/p2\n" {
		t.Errorf("the registry holds %q, wanted one line per project", got)
	}
}

// The whole reason reading and pruning are one call. A caller able to read without pruning gets the
// stale answer, and that answer blocks a removal for a project the machine has already lost.
func TestAReadPrunesTheProjectsThatAreGone(t *testing.T) {
	t.Parallel()
	f, run := newRegistryFixture(t)
	run.RecordInstall(f.base + "/p1")
	run.RecordInstall(f.base + "/p2")
	f.RemoveAll(f.base + "/p2")

	live := run.LiveInstalls()

	if len(live) != 1 || live[0] != f.base+"/p1" {
		t.Errorf("a live read answered %v, wanted only %s", live, f.base+"/p1")
	}
	if strings.Contains(registryBody(t, run), "/p2") {
		t.Errorf("the project whose directory is gone is still on disk: %q", registryBody(t, run))
	}
}

// A human who opens this file to see what is in it may well annotate it. A note eaten during a prune
// of dead projects is a poor answer to a question they did not ask.
func TestACommentIsNotReadBackAsAProjectAndSurvivesThePrune(t *testing.T) {
	t.Parallel()
	f, run := newRegistryFixture(t)
	run.RecordInstall(f.base + "/p1")
	run.RecordInstall(f.base + "/p2")
	f.Write(run.RegistryFile(), registryBody(t, run)+"# a note someone left\n")
	f.RemoveAll(f.base + "/p2")

	live := run.LiveInstalls()

	for _, project := range live {
		if strings.Contains(project, "a note someone left") {
			t.Errorf("a comment was answered as a project: %v", live)
		}
	}
	if !strings.Contains(registryBody(t, run), "a note someone left") {
		t.Errorf("the note did not survive the prune: %q", registryBody(t, run))
	}
}

func TestForgettingAProjectIsIdempotent(t *testing.T) {
	t.Parallel()
	f, run := newRegistryFixture(t)
	run.RecordInstall(f.base + "/p1")
	run.RecordInstall(f.base + "/p2")

	run.ForgetInstall(f.base + "/p1")
	f.ExpectSaid("forgot")
	if strings.Contains(registryBody(t, run), "/p1\n") {
		t.Errorf("the forgotten project is still recorded: %q", registryBody(t, run))
	}
	// The control: forgetting one project leaves the other.
	if !strings.Contains(registryBody(t, run), "/p2\n") {
		t.Errorf("forgetting one project took the other with it: %q", registryBody(t, run))
	}

	run.ForgetInstall(f.base + "/p1")
	f.ExpectSaid("was not recorded")
	f.expectRefusals(run, 0)
}

// A missing registry is no error. It is a machine that has installed into no project.
func TestAMachineWithNoRegistryReadsCleanAndForgetsCleanly(t *testing.T) {
	t.Parallel()
	f, run := newRegistryFixture(t)

	if live := run.LiveInstalls(); len(live) != 0 {
		t.Errorf("a machine with no registry named %v, wanted nothing", live)
	}
	run.ForgetInstall(f.base + "/p1")

	f.ExpectSaid("nothing recorded to forget")
	f.expectRefusals(run, 0)
}

func TestADryRunWritesNoRegistry(t *testing.T) {
	t.Parallel()
	f := newBareFixture(t)
	f.MkdirAll(f.base + "/p1")
	run := f.newRun(installer.RunOptions{ConfigHome: f.base + "/cfg", DryRun: true})

	run.RecordInstall(f.base + "/p1")

	f.ExpectSaid("would record")
	f.ExpectAbsent(run.RegistryFile())
}
