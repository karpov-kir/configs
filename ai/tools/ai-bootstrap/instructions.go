package aibootstrap

import (
	"bytes"
	"os"
	"strings"

	"configs/ai/tools/flavor"
	"configs/ai/tools/shell"
)

// The owner tier's memory store, and where an owner install before this one put it. The spelling was
// `Document` for months, which no other tool on a Mac uses, and the owner's own instructions name
// `Documents`. A fix to the destination alone would leave every entry already written at a path no
// session reads, so moveLegacyOwnerMemory, a step of the install run, moves them.
const (
	ownerMemoryFile = "/Documents/AI/MEMORY.md"
	legacyMemoryOne = "/Document/AI/MEMORY.md"
)

// A Codex profile can shadow AGENTS.md with a file of its own. A run that wrote the instructions
// anyway would leave the tree installed with no client loading it. The question comes before the
// first write, and an uninstall is exempt, because it removes what the shadow hides.
func (run *invocation) shadowedInstructions() string {
	if run.agent != codexAgent || run.isUninstall {
		return ""
	}
	if !isNonEmptyFile(run.CodexHome + "/AGENTS.override.md") {
		return ""
	}
	return run.CodexHome + "/AGENTS.override.md shadows AGENTS.md — merge its instructions before installing"
}

func isNonEmptyFile(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir() && info.Size() > 0
}

// The instruction step, and whether it finished. The rtk step reads the answer.
func (run *invocation) writeInstructions() bool {
	run.mounting.Say("instructions")
	if run.isOwner {
		return run.writeOwnerInstructions() && run.moveLegacyOwnerMemory() && run.ensureOwnerMemory()
	}
	// Asked again here as well as at the top of the run. do() stops a run before it writes anything.
	// This call guards the write itself, so a path that reached here some other way still cannot write
	// into a file no client will load.
	if refusal := run.shadowedInstructions(); refusal != "" {
		run.mounting.Refuse(refusal)
		return false
	}
	if !shell.PathExists(run.instructionFile) && !shell.IsSymlink(run.instructionFile) {
		if run.isDryRun {
			run.mounting.Say("  would create " + run.instructionFile + " and add the kk-flavor region")
			return true
		}
		if !run.createEmptyInstructionFile() {
			return false
		}
	}
	return run.mounting.WriteRegion(run.instructionFile, flavor.RegionOpen, flavor.RegionClose, flavor.RegionBody)
}

// The only path that creates an instruction file. The region writer refuses a missing file on
// purpose, because a typo in a path would otherwise produce a plausible new file in someone's
// repository. A client that has never been configured is handed an empty file here first.
func (run *invocation) createEmptyInstructionFile() bool {
	directory := shell.DirName(run.instructionFile)
	if !shell.IsDir(directory) {
		if err := os.MkdirAll(directory, 0o755); err != nil {
			run.mounting.Refuse("could not create " + directory + ", so the instruction region was not written")
			return false
		}
	}
	if err := os.WriteFile(run.instructionFile, nil, 0o644); err != nil {
		run.mounting.Refuse("could not create " + run.instructionFile + ", so the instruction region was not written")
		return false
	}
	return true
}

// --- the owner tier -------------------------------------------------------------------------------

// The owner gets the whole owner template as a copy, so the file is this repository's and a later
// edit of it is the owner's. A whole-file replacement is destructive in a way a region never is, and
// every refusal here guards that. The copy goes in only where this repository wrote what is already
// there, and a backup is taken whenever the bytes differ.
func (run *invocation) writeOwnerInstructions() bool {
	source := run.ownerSource()
	if !shell.IsRegularFile(source) || shell.IsSymlink(source) {
		run.mounting.Refuse(source + " must be a regular owner instruction source")
		return false
	}
	receipt := run.ownerReceipt()
	if shell.IsSymlink(receipt) || (shell.PathExists(receipt) && !shell.IsRegularFile(receipt)) {
		run.mounting.Refuse(receipt + " must be a regular installation receipt")
		return false
	}
	if (shell.PathExists(run.instructionFile) || shell.IsSymlink(run.instructionFile)) && !run.ownerFileMatches() {
		run.mounting.Refuse(run.instructionFile +
			" contains personal instructions — preserve them before replacing it with the owner copy")
		return false
	}
	if run.isDryRun {
		run.mounting.Say("  would install " + source + " as a regular copy at " + run.instructionFile)
		return true
	}
	if !run.installOwnerCopy(source) {
		return false
	}
	// The receipt is what a later run compares against when the source has moved on. A run lacking one
	// reads an upgraded owner template as a file the owner edited, and refuses forever.
	if !sameBytes(source, receipt) {
		if err := copyFile(source, receipt); err != nil {
			run.mounting.Refuse("could not record installed owner instructions")
			return false
		}
	}
	run.mounting.Say("  ok       " + run.instructionFile + " is an independent owner copy")
	return true
}

func (run *invocation) ownerSource() string {
	return run.Repo + "/owner-instructions.md"
}

func (run *invocation) ownerReceipt() string {
	return run.instructionFile + ".kk-flavor-installed"
}

// The copy is written through a temp file beside the target and renamed, so a killed run leaves the
// previous copy whole. A symlink at the target is replaced. An older install mounted this
// repository's own file there, and a write through that link would edit the checkout.
func (run *invocation) installOwnerCopy(source string) bool {
	directory := shell.DirName(run.instructionFile)
	if err := os.MkdirAll(directory, 0o755); err != nil {
		run.mounting.Refuse("could not create instruction directory")
		return false
	}
	if !shell.IsSymlink(run.instructionFile) && sameBytes(run.instructionFile, source) {
		return true
	}
	if shell.PathExists(run.instructionFile) {
		backup, err := backupOf(run.instructionFile)
		if err != nil {
			run.mounting.Refuse("could not back up " + run.instructionFile)
			return false
		}
		run.mounting.Say("  backup   " + backup)
	}
	if err := replaceWithCopy(source, run.instructionFile); err != nil {
		run.mounting.Refuse("could not install owner instructions")
		return false
	}
	return true
}

// Whether what is at the instruction file is something this repository put there. Each shape it
// accepts is a machine an earlier version of this installer left behind. Anything else is the owner's
// own writing, and the caller refuses it.
func (run *invocation) ownerFileMatches() bool {
	// What the first owner installs mounted: a link to this checkout's own instruction file or to the
	// owner template.
	if shell.IsSymlink(run.instructionFile) {
		value, err := os.Readlink(run.instructionFile)
		if err != nil {
			return false
		}
		return value == run.Repo+"/CLAUDE.md" || value == run.Repo+"/AGENTS.md" || value == run.ownerSource()
	}
	if !shell.IsRegularFile(run.instructionFile) {
		return false
	}
	if sameBytes(run.instructionFile, run.ownerSource()) {
		return true
	}
	// The template as it was when this machine was installed, which the receipt records. That is the
	// only way an upgraded template is told from a file the owner edited.
	receipt := run.ownerReceipt()
	if shell.IsRegularFile(receipt) && !shell.IsSymlink(receipt) && sameBytes(run.instructionFile, receipt) {
		return true
	}
	// For Codex, the generated region this repository used to write, with or without the RTK note that
	// sat under it.
	if run.agent != codexAgent {
		return false
	}
	return run.matchesGeneratedCodexInstructions()
}

// The generated file an older Codex install wrote: the fenced region, optionally followed by the RTK
// note that install also added. The comparison drops blank lines from both sides, because the two
// halves were assembled by different runs and the owner chose no part of the spacing between them.
func (run *invocation) matchesGeneratedCodexInstructions() bool {
	body, err := os.ReadFile(run.instructionFile)
	if err != nil {
		return false
	}
	actual := withoutBlankLines(string(body))
	generated := withoutBlankLines(flavor.RegionOpen + "\n" + flavor.RegionBody + "\n" + flavor.RegionClose + "\n")
	if actual == generated {
		return true
	}
	return actual == generated+"\n"+withoutBlankLines(run.legacyRtkNote())
}

// What the older Codex install appended under the region. The text is reproduced here because it
// names CODEX_HOME by an absolute path, and the exact text on any one machine exists only in that
// machine's file.
func (run *invocation) legacyRtkNote() string {
	return "@" + run.CodexHome + "/RTK.md\n\n" +
		flavor.LegacyRtkRegionOpen + "\n" +
		"Read `" + run.CodexHome + "/RTK.md` for RTK usage. For unsupported commands or exact output, " +
		"use `rtk proxy <command>`.\n" +
		flavor.LegacyRtkRegionClose
}

func withoutBlankLines(text string) string {
	var kept []string
	for _, line := range shell.SplitLines(text) {
		if strings.TrimLeft(line, shell.SpaceBytes) != "" {
			kept = append(kept, line)
		}
	}
	return strings.Join(kept, "\n")
}

// The owner's entries, moved off the path an earlier install wrote them to. A run that skipped this
// leaves them where no session reads, and the failure is silent: the run reports a memory file
// created, and that file is empty.
//
// Returns true when there is no entry to move, which is every run after the first.
func (run *invocation) moveLegacyOwnerMemory() bool {
	legacy, memory := run.Home+legacyMemoryOne, run.Home+ownerMemoryFile
	if !shell.PathExists(legacy) {
		return true
	}
	// Two stores, with no way to tell which holds what. The merge is the human's call. This code
	// cannot read either store, and it cannot tell which entry is newer.
	if shell.PathExists(memory) {
		run.mounting.Refuse("owner memory exists at both " + legacy + " and " + memory +
			" — merge them into " + memory + " and remove " + legacy)
		return false
	}
	if run.isDryRun {
		run.mounting.Say("  would move " + legacy + " to " + memory)
		// A dry run moves no file. ensureOwnerMemory, the step that follows, then finds the destination
		// absent and says it would create one. That is two lines that cannot both hold, about the file
		// this exists to protect.
		return false
	}
	if err := os.MkdirAll(shell.DirName(memory), 0o755); err != nil {
		run.mounting.Refuse("could not create " + shell.DirName(memory) + ", so owner memory was left at " + legacy)
		return false
	}
	// A hard link and then a remove. `os.Rename` REPLACES a destination that appeared since the
	// PathExists check, and that check is not atomic with the move. A session following the very rule
	// this installer writes can create the destination in between. `os.Link` refuses an existing
	// destination, so the only copy there is survives this line.
	if err := os.Link(legacy, memory); err != nil {
		run.mounting.Refuse("could not move owner memory from " + legacy + " to " + memory +
			" — both were left as they are")
		return false
	}
	if err := os.Remove(legacy); err != nil {
		run.mounting.Refuse("owner memory was copied to " + memory + " and " + legacy +
			" could not be removed — merge them by hand, since a later run will refuse both")
		return false
	}
	// Only if they are now empty, since a directory holding anything else is the human's. A failure
	// here is ignored, because a leftover empty directory changes no outcome.
	os.Remove(shell.DirName(legacy))
	os.Remove(shell.DirName(shell.DirName(legacy)))
	run.mounting.Say("  moved    " + legacy + " to " + memory)
	return true
}

// The owner's memory store, created empty where it is missing and never written over. Its whole
// content is the owner's, and this is the only file in the install of which that is true.
func (run *invocation) ensureOwnerMemory() bool {
	memory := run.Home + ownerMemoryFile
	if !shell.PathExists(memory) {
		if run.isDryRun {
			run.mounting.Say("  would create " + memory)
			return true
		}
		if err := os.MkdirAll(shell.DirName(memory), 0o755); err != nil {
			run.mounting.Refuse("could not create owner memory at " + memory)
			return false
		}
		// O_EXCL, so a file that appeared between the PathExists check and this write is kept whole.
		// This installer never overwrites the owner's memory, and that is the whole point of the file.
		file, err := os.OpenFile(memory, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
		if err != nil {
			run.mounting.Refuse("could not create owner memory at " + memory)
			return false
		}
		_, writeErr := file.WriteString("# Memory\n")
		if closeErr := file.Close(); writeErr != nil || closeErr != nil {
			run.mounting.Refuse("could not create owner memory at " + memory)
			return false
		}
		return true
	}
	if run.isDryRun {
		return true
	}
	if !shell.IsRegularFile(memory) || !isReadableAndWritable(memory) {
		run.mounting.Refuse(memory + " must be a readable, writable memory file")
		return false
	}
	return true
}

// --- removing what the owner tier installed -------------------------------------------------------

// Removed only when what is there is still this repository's copy. An owner who edited it gets a
// refusal and keeps their file. An uninstall takes back what this installer put there, and an edited
// file is no longer that.
func (run *invocation) removeOwnerInstructions() bool {
	if shell.PathExists(run.instructionFile) || shell.IsSymlink(run.instructionFile) {
		if !run.ownerFileMatches() {
			run.mounting.Refuse(run.instructionFile + " was modified — owner instructions were preserved")
			return false
		}
		if run.isDryRun {
			run.mounting.Say("  would remove the owner copy at " + run.instructionFile)
		} else if err := os.Remove(run.instructionFile); err != nil {
			run.mounting.Refuse("could not remove owner instructions")
			return false
		}
	}
	receipt := run.ownerReceipt()
	if !run.isDryRun && shell.IsRegularFile(receipt) && !shell.IsSymlink(receipt) {
		if err := os.Remove(receipt); err != nil {
			run.mounting.Refuse("could not remove owner receipt")
			return false
		}
	}
	return true
}

// --- reading and copying files ----------------------------------------------------------------------

// Whether two files hold the same bytes. A file that cannot be read counts as different, which is
// what every caller here wants: it sends them down the write path.
func sameBytes(one, other string) bool {
	first, err := os.ReadFile(one)
	if err != nil {
		return false
	}
	second, err := os.ReadFile(other)
	if err != nil {
		return false
	}
	return bytes.Equal(first, second)
}

func copyFile(source, target string) error {
	body, err := os.ReadFile(source)
	if err != nil {
		return err
	}
	return os.WriteFile(target, body, 0o644)
}

// A copy beside the original, under a unique name no other writer uses. The mode is carried over,
// which is what `cp -p` did and what makes the backup readable to whoever could read the original.
func backupOf(file string) (string, error) {
	body, err := os.ReadFile(file)
	if err != nil {
		return "", err
	}
	backup, err := os.CreateTemp(shell.DirName(file), shell.BaseName(file)+".backup.")
	if err != nil {
		return "", err
	}
	name := backup.Name()
	if _, err = backup.Write(body); err != nil {
		backup.Close()
		os.Remove(name)
		return "", err
	}
	if err = backup.Close(); err != nil {
		os.Remove(name)
		return "", err
	}
	if info, statErr := os.Stat(file); statErr == nil {
		os.Chmod(name, info.Mode().Perm())
	}
	return name, nil
}

// Through a temp file in the target's own directory, then renamed. One filesystem, so the rename is
// atomic and a killed run never leaves a half-written instruction file.
func replaceWithCopy(source, target string) error {
	body, err := os.ReadFile(source)
	if err != nil {
		return err
	}
	staged, err := os.CreateTemp(shell.DirName(target), shell.BaseName(target)+".tmp.")
	if err != nil {
		return err
	}
	name := staged.Name()
	if _, err = staged.Write(body); err != nil {
		staged.Close()
		os.Remove(name)
		return err
	}
	if err = staged.Close(); err != nil {
		os.Remove(name)
		return err
	}
	if err = os.Rename(name, target); err != nil {
		os.Remove(name)
		return err
	}
	return nil
}
