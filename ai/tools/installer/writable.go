package installer

import (
	"fmt"

	"configs/ai/tools/shell"
)

// FileWritable is whether a file that already exists is one this run may write in place, refusing
// with the reason when it is not. A caller runs it before it hands such a file to another program.
// The Codex RTK step does, because rtk would write over it.
//
// A symlink is refused, because a write through one edits a file in a place the caller never named,
// and for an instruction file symlinked into a checkout that means editing the checkout. A missing
// file is refused rather than assumed, so a typo in a path cannot pass.
func (r *Run) FileWritable(file string) bool {
	if shell.IsSymlink(file) {
		r.Refuse(file + " is a symlink, and this writes into the file itself — repoint or remove it, then re-run")
		return false
	}
	if !shell.PathExists(file) {
		r.Refuse(file + " does not exist, and this never creates one — nothing was written")
		return false
	}
	if !shell.IsRegularFile(file) {
		r.Refuse(file + " is not a regular file — nothing was written")
		return false
	}
	if !isWritable(file) {
		r.Refuse(file + " is not writable — nothing was written")
		return false
	}
	links, err := r.tree.linkCount(file)
	if err != nil {
		r.Refuse(file + ": no link count could be read, so whether its contents are shared is unknown — nothing was written")
		return false
	}
	if links > 1 {
		r.Refuse(fmt.Sprintf("%s has %d hard links, so its contents are shared with a file this never named — nothing was written", file, links))
		return false
	}
	return true
}
