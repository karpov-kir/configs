package ecocheck_test

// The mount findings echo a path resolved through $HOME, which the reviewed branch does not choose —
// so this is defence in depth rather than a branch-reachable hole. It is still the one mount message
// built from text nothing in this checker wrote, and an ESC sequence in it erases the finding printed
// above it in the transcript a reviewing agent drafts its comments from.

import "testing"

func TestMountFindingCarriesNoControlByte(t *testing.T) {
	// The directory sits outside the root on purpose: inside it, the walk would name it too, and the
	// case would then pass on a sanitiser somewhere else entirely.
	newMountElsewhere := func(t *testing.T) *fixture {
		f := newRoot(t)
		f.newHomeWithoutFlavorMount()
		elsewhere := f.base + "/elsewhere\x1b[2Kflavor"
		f.mkdirAll(elsewhere)
		f.symlink(elsewhere, f.home+"/.kk-flavor")
		return f
	}

	// Without this the case below passes on a run that raised no mount finding at all.
	t.Run("reports a flavor mounted somewhere else (control for the case below)", func(t *testing.T) {
		newMountElsewhere(t).reports("flavor mounted elsewhere")
	})

	t.Run("and no control byte reaches the output", func(t *testing.T) {
		newMountElsewhere(t).doesNotReport("\x1b")
	})
}
