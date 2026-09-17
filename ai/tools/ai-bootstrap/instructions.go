package aibootstrap

import (
	"bytes"
	"os"
	"strings"

	"configs/ai/tools/flavor"
	"configs/ai/tools/shell"
)

// The owner tier's memory store, and where an owner install before this one put it. The spelling was
// `Document` for months, which is not a directory anything else on a Mac uses, and the owner's own
// instructions name `Documents`. Correcting the destination alone would leave every entry already
// written sitting at a path no session reads, so the migration below moves them.
const (
	ownerMemoryFile = "/Documents/AI/MEMORY.md"
	legacyMemoryOne = "/Document/AI/MEMORY.md"
)

// A Codex profile can shadow AGENTS.md with a file of its own, and a run that wrote the instructions
// anyway would leave the tree installed and nothing loading it. Asked before anything is written; an
// uninstall is exempt, since it is removing what the shadow hides rather than relying on it.
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

// The instruction step, and whether it finished. The answer is read by the rtk step: initialising rtk
// against an instruction file this run refused to write leaves the machine describing tooling whose
// instructions never arrived.
func (run *invocation) writeInstructions() bool {
	run.mounting.Say("instructions")
	if run.isOwner {
		return run.writeOwnerInstructions() && run.moveLegacyOwnerMemory() && run.ensureOwnerMemory()
	}
	// Asked again here as well as at the top of the run. The check above stops a run before it writes
	// anything; this one is the guard on the write itself, so a path that reached here some other way
	// still cannot write into a file nothing will load.
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

// The one path that creates an instruction file. The region writer refuses a file that is not there,
// deliberately — a typo in a path would otherwise produce a plausible-looking new file in someone's
// repository — so a client that has never been configured is handed an empty one here first.
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

// The owner gets a copy of ai/owner-instructions.md rather than a region inside their own file, so the
// whole file is this repository's and a later edit of it is the owner's.
//
// That makes replacing it destructive in a way a region never is, which is what every refusal below
// guards: the copy goes in only when what is there is one this repository wrote, and a backup is taken
// on the way whenever the bytes differ.
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
	// The receipt is what a later run compares against when the source has moved on: without it an
	// upgrade of owner-instructions.md would read as a file the owner edited, and refuse forever.
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

// Written through a temp file beside the target and renamed, so a killed run leaves the previous copy
// whole rather than a half-written instruction file. A symlink at the target is replaced rather than
// written through: an older install mounted this repository's own file there, and writing through that
// link would edit the checkout.
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

// Whether what is at the instruction file is something this repository put there. Four shapes count,
// and every one of them is a machine an earlier version of this installer left behind:
//
//   - a symlink to this checkout's own instruction file or to the owner template, which is what the
//     first owner installs mounted;
//   - a copy of the current template;
//   - a copy of the template as it was when this machine was installed, which is what the receipt
//     records and the only way an upgraded template is told from an edited file;
//   - for Codex, the generated region this repository used to write, with or without the RTK note that
//     sat under it.
//
// Anything else is the owner's own writing and is refused rather than replaced.
func (run *invocation) ownerFileMatches() bool {
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
	receipt := run.ownerReceipt()
	if shell.IsRegularFile(receipt) && !shell.IsSymlink(receipt) && sameBytes(run.instructionFile, receipt) {
		return true
	}
	if run.agent != codexAgent {
		return false
	}
	return run.matchesGeneratedCodexInstructions()
}

// The generated file an older Codex install wrote: the fenced region, optionally followed by the RTK
// note that install also added. Compared with blank lines dropped from both sides, because the two
// were assembled by different runs and the number of blank lines between them is not something the
// owner chose.
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

// What the older Codex install appended under the region. Reproduced rather than recognised by a
// fence: the note names CODEX_HOME by absolute path, so the file on any one machine is the only place
// its exact text exists.
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

// The owner's memory store, created empty when it is not there and never written over: it is the one
// file in this install whose whole content is the owner's.
// The owner's entries, moved off the path an earlier install wrote them to. Drop this and they stay
// somewhere no session reads, which is silent: the run reports a memory file created and the file it
// created is empty.
//
// Returns true when there is nothing to move, which is every run after the first.
func (run *invocation) moveLegacyOwnerMemory() bool {
	legacy, memory := run.Home+legacyMemoryOne, run.Home+ownerMemoryFile
	if !shell.PathExists(legacy) {
		return true
	}
	// Two stores and no way to tell which holds what. Merging them is the human's call — this one
	// cannot read either and cannot know which entry is newer.
	if shell.PathExists(memory) {
		run.mounting.Refuse("owner memory exists at both " + legacy + " and " + memory +
			" — merge them into " + memory + " and remove " + legacy)
		return false
	}
	if run.isDryRun {
		run.mounting.Say("  would move " + legacy + " to " + memory)
		// Nothing moved, so ensureOwnerMemory below would still find the destination absent and say it
		// would create one — two lines that cannot both hold, about the file this exists to protect.
		return false
	}
	if err := os.MkdirAll(shell.DirName(memory), 0o755); err != nil {
		run.mounting.Refuse("could not create " + shell.DirName(memory) + ", so owner memory was left at " + legacy)
		return false
	}
	// A hard link and then a remove, rather than a rename: `os.Rename` REPLACES a destination that
	// appeared since the check above, and the check is not atomic with the move — a session following
	// the very rule this installer writes can create it in between. `os.Link` refuses an existing
	// destination, so the only copy there is cannot be replaced by this line.
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
	// Only if they are now empty; a directory holding anything else is the human's. Best-effort, since
	// a leftover empty directory decides nothing.
	os.Remove(shell.DirName(legacy))
	os.Remove(shell.DirName(shell.DirName(legacy)))
	run.mounting.Say("  moved    " + legacy + " to " + memory)
	return true
}

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
		// O_EXCL, so a file that appeared between the check above and this write is kept rather than
		// truncated. The whole point of this file is that nothing here ever overwrites it.
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
// refusal and keeps their file: the uninstall's job is to take back what this put there, and by then
// the file is no longer that.
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

// Whether two files hold the same bytes. A file that cannot be read is not the same as one that can,
// which is the answer every caller here wants: it sends them down the write path rather than the
// leave-it-alone one.
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

// A dated copy beside the original, under a name nothing else writes. The mode is carried over, which
// is what `cp -p` did and what makes the backup readable to whoever could read the original.
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

// Through a temp file in the target's own directory, then renamed — one filesystem, so the rename is
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
