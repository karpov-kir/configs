package installer

import (
	"fmt"
	"strings"

	"configs/ai/tools/shell"
)

// AddConfig declares one mount for Mount to write later.
//
// The guard reports the two lists differently. A config is individually consequential — a shell, a
// git identity, the instructions every agent session loads — and the guard names it. A bulk set is
// homogeneous, and the guard prints its count.
func (r *Run) AddConfig(source, target string) {
	r.configs = append(r.configs, Mount{Source: source, Target: target})
}

func (r *Run) AddBulk(source, target string) {
	r.bulk = append(r.bulk, Mount{Source: source, Target: target})
}

func (r *Run) BulkMounts() []Mount {
	return r.bulk
}

// The project installer is the caller. It declares its skills against this checkout, which gives
// mountForeignRoot a checkout to recognise. It then points them at the shared bucket, and every
// install of this flavor holds the same path. A declaration at the bucket up front would leave the
// guard blind and a project quietly repointed off somebody else's clone.

// RewriteBulkSources replaces every declared bulk source with what rewrite answers for it.
func (r *Run) RewriteBulkSources(rewrite func(source string) string) {
	for i := range r.bulk {
		r.bulk[i].Source = rewrite(r.bulk[i].Source)
	}
}

// Link writes over an existing symlink, since a symlink carries no data of its own. Any other
// existing target is refused. mountForeignRoot covers the case where that reasoning holds for the
// link but fails for what the link is part of.

// The project installer is why this is exported. It mounts a bucket that every other mount in its
// table then resolves THROUGH, and that bucket has to land before the rest are even spelled. Mount
// still writes the table, and it links the bucket again and reports it as already ok.

// Link writes one mount at once, and reports which of the four states it found.
func (r *Run) Link(source, target string) bool {
	if !shell.PathExists(source) {
		r.Refuse(source + " is missing from the repository, so " + target + " was left alone")
		return false
	}
	if shell.IsSymlink(target) {
		if linkValue(target) == strings.TrimSuffix(source, "/") {
			r.Say("  ok       " + target)
			return true
		}
		return r.apply(change{
			would: "would repoint " + target + " -> " + source,
			did:   "repointed " + target,
			write: func() string {
				if err := r.tree.symlink(source, target); err != nil {
					return "could not repoint " + target + " at " + source + ": " + err.Error()
				}
				return ""
			},
		})
	}
	if shell.PathExists(target) {
		// The whole reason a bootstrap script is not env/README.md's `rm -rf`.
		r.Refuse(target + " exists and is not a symlink — move it aside, then re-run")
		return false
	}
	// Under a trim of the last component, a target ending in a slash names itself as its own parent.
	// The parent created there is a directory the link then lands inside, and shell.DirName avoids it.

	// An empty $HOME leaves a root-level target whose parent is `/`. A computation that answered empty
	// there would refuse while naming a parent it cannot print, sending the reader after a directory
	// that was never the problem.
	parent := shell.DirName(target)
	return r.apply(change{
		would: "would link " + target + " -> " + source,
		did:   "linked   " + target,
		write: func() string {
			if !shell.IsDir(parent) {
				if err := r.tree.mkdirAll(parent); err != nil {
					return "could not create " + parent + ": " + err.Error()
				}
			}
			if err := r.tree.symlink(source, target); err != nil {
				return "could not link " + target + " at " + source + ": " + err.Error()
			}
			return ""
		},
	})
}

// --- is this machine already mounted somewhere else? ----------------------------------------------

// Link treats an existing symlink as safe to write over, since a symlink carries no data of its own.
// That holds for the link. The damage guarded here is to the MOUNT. A live link can name a checkout
// that is real and holds this machine's actual config, and then the link is current and this
// checkout is the stranger. A repoint there moves the human's whole setup to this clone.

// Delete the clone afterwards, which is the entire point of a scratch clone, and their next login
// is missing .zshrc, .gitconfig and every agent instruction. This code is contracted to refuse a
// write it cannot justify, and it would have carried out that deletion while printing "repointed"
// thirty-odd times.

// The guard is narrow on purpose, so the rule Link states keeps working. A mount belongs to another
// checkout on two conditions. Its link value ends in the same relative path, `…/zsh/.zshrc` for
// `~/.zshrc`, and the root left over when that path is stripped holds a copy of the calling
// installer.

// Together those two make it the same file in a second copy of this repository. An unrelated config
// that the stale-symlink rule is right to repoint fails the second condition. A README-era link
// with a trailing slash still compares equal to this checkout and stays the compatibility case it
// was.

// A root that no longer resolves fails the test too. That is the aftermath of this bug, or of a
// checkout moved on purpose, and repointing a dangling link is the repair.

// Returns the root of the second checkout this target is already mounted from, or empty.
func (r *Run) mountForeignRoot(source, target string) string {
	if !shell.IsSymlink(target) {
		return ""
	}
	current := linkValue(target)
	// Absolute only. A relative link value resolves against the link's own directory, and a root named
	// from it would come out of this process's working directory instead. Every link written here is
	// absolute, so a relative value came from elsewhere and sits outside this run's mounts.
	if !strings.HasPrefix(current, "/") {
		return ""
	}
	relative := strings.TrimPrefix(source, r.repo+"/")
	if !strings.HasSuffix(current, "/"+relative) {
		return ""
	}
	root := realDir(strings.TrimSuffix(current, "/"+relative))
	if root == "" || root == r.repo {
		return ""
	}
	// The root has to hold a copy of the calling installer. A matching path component alone is too
	// weak, because several sources are one component long, among them `nvim`, `ghostty` and
	// `kk-flavor`.

	// The tail comparison alone reads `~/.config/nvim -> ~/.dotfiles/nvim` as a second checkout. It then
	// refuses the whole run and tells the human their configuration is mounted from a directory that
	// never held it. That link is an ordinary stale mount, and Link is right to repoint it.
	if !shell.IsRegularFile(root + "/" + r.scriptName) {
		return ""
	}
	return root
}

func (r *Run) noteForeignRoot(candidate string) {
	for _, known := range r.foreignRoots {
		if known == candidate {
			return
		}
	}
	r.foreignRoots = append(r.foreignRoots, candidate)
}

// Returns how many mounts a root holds, spelled the way the guard reports it. With a bulk set the
// count leads and the kinds are broken out. A run with no bulk set has a single kind to report.
func (r *Run) mountTally(named, bulk int) string {
	if bulk > 0 {
		return fmt.Sprintf("%d mounts (%d configs and %d %s)", named+bulk, named, bulk, r.bulkLabel)
	}
	return fmt.Sprintf("%d mounts", named+bulk)
}

// The count leads, before any list. Every line of the list carries the same root, so the list is one
// fact repeated. The scale is what a reader cannot reconstruct, and a reader who takes in the named
// configs and stops has missed that every skill moves too.

// Prints one root's worth of the refusal: what resolves to it, what running from here would do to
// them, and the refusal line the exit code carries.
func (r *Run) refuseForeignRoot(root string) {
	var named []string
	for i, mount := range r.configs {
		if r.configForeign[i] == root {
			named = append(named, mount.Target)
		}
	}
	bulkCount := 0
	for i := range r.bulk {
		if r.bulkForeign[i] == root {
			bulkCount++
		}
	}

	tally := r.mountTally(len(named), bulkCount)
	r.Say("")
	r.Say("  " + tally + " currently resolve to")
	r.Say("    " + root)
	r.Say("  and running from here would move every one of them to")
	r.Say("    " + r.repo)
	r.Say("")
	for _, target := range named {
		r.Say("    " + target)
	}
	if bulkCount > 0 {
		r.Say(fmt.Sprintf("    ...and all %d %s", bulkCount, r.bulkLabel))
	}
	r.Say("")
	r.Refuse(tally + " resolve to " + root + ", not to this checkout — nothing was written; " +
		"re-run with --relocate to move them here")
}

// One call, because the survey has to cover every mount before the first one is written. A caller
// able to skip the survey is a caller able to skip the guard.

// Mount answers false when the guard stopped it before the first write. That is the caller's cue to
// report and exit, leaving the steps that follow mounting undone.

// Mount surveys every mount, refuses if this machine's config lives in another checkout, then links.
func (r *Run) Mount() bool {
	r.Say("mounts")
	foreignTotal := r.surveyForeignMounts()

	switch {
	case foreignTotal == 0:
		// A silent guard reads exactly like a guard that was never reached, and this one runs on every
		// machine that is already set up. The pass is announced out loud on the way past.
		r.Say("  ok       no mount on this machine comes from another checkout")
	case r.relocate:
		r.Say(fmt.Sprintf("  --relocate: moving %d mount(s) to this checkout, from:", foreignTotal))
		for _, root := range r.foreignRoots {
			r.Say("    " + root)
		}
	default:
		r.Say("")
		r.Say("  This checkout is not where " + r.mountScopeLabel + " is mounted.")
		for _, root := range r.foreignRoots {
			r.refuseForeignRoot(root)
		}
		return false
	}

	r.Say("links")
	for _, mount := range r.configs {
		r.Link(mount.Source, mount.Target)
	}
	if len(r.bulk) > 0 {
		r.Say(r.bulkLabel)
		for _, mount := range r.bulk {
			r.Link(mount.Source, mount.Target)
		}
	}

	// The prune sits inside Mount, so a caller cannot skip the scan and keep an unreachable mount. It
	// sits after the links, because a machine mounted from another checkout writes no files at all,
	// removals included.

	// pruneStaleMounts, NOT Unmount. This drops mounts with a missing SOURCE. Unmount removes every
	// mount the run declared. They arrived from two branches under one name, and a normal install
	// calling Unmount links the whole tree and then tears it down.
	r.pruneStaleMounts()
	return true
}

func (r *Run) surveyForeignMounts() int {
	r.configForeign = make([]string, len(r.configs))
	r.bulkForeign = make([]string, len(r.bulk))
	foreignTotal := 0
	for i, mount := range r.configs {
		r.configForeign[i] = r.mountForeignRoot(mount.Source, mount.Target)
		if r.configForeign[i] != "" {
			foreignTotal++
			r.noteForeignRoot(r.configForeign[i])
		}
	}
	for i, mount := range r.bulk {
		r.bulkForeign[i] = r.mountForeignRoot(mount.Source, mount.Target)
		if r.bulkForeign[i] != "" {
			foreignTotal++
			r.noteForeignRoot(r.bulkForeign[i])
		}
	}
	return foreignTotal
}

// --- taking it back out ---------------------------------------------------------------------------

// The caller names the target instead of Mount taking it off the table, because one caller has a
// target the table never held. The Codex install migrates a skill out of an older mount directory
// once its replacement exists.

// Removal lives here because the proof has to run for every caller. A target is removed only if it
// is a symlink AND its value resolves under the checkout. Anything else — a real file, a symlink
// into somebody else's checkout, a link that resolves nowhere — is reported and left. Link makes the
// same promise at the other end.

// ai/README.md performed this proof by hand, with a `readlink` case matching a path fragment. A test
// against the checkout cannot mistake a similarly-named directory for this one.

// A target that is already gone counts as success. Uninstall run twice is ordinary.

// UnmountTarget removes one mount, and only when this checkout can prove it wrote it.
func (r *Run) UnmountTarget(target string) bool {
	if !shell.IsSymlink(target) {
		if shell.PathExists(target) {
			r.Refuse(target + " is not a symlink, so this did not write it — remove it yourself if you mean to")
			return false
		}
		r.Say("  ok       " + target + " is already gone")
		return true
	}
	current, err := readLink(target)
	if err != nil {
		r.Refuse("could not read where " + target + " points, so whether this wrote it is unknown — left alone")
		return false
	}
	// Absolute only, the same test mountForeignRoot makes and for the same reason. A relative value
	// resolves against THIS process's working directory instead of the link's own, and ownership then
	// gets judged from somewhere the link never named. A link reading `notmine` resolves under the
	// checkout and is deleted.

	// Every link written here is absolute, so a relative value came from elsewhere and this run leaves
	// it alone.
	if !strings.HasPrefix(current, "/") {
		r.Refuse(target + " points at the relative path " + current + ", which this never writes — left alone")
		return false
	}
	// The value is resolved before it is compared, and a link written through a differently-spelled but
	// equivalent path is still recognised as this checkout's. A link resolving nowhere resolves to
	// empty and falls through to the refusal. Its target is unknown, so its ownership is unknown too.
	resolved := realDir(shell.DirName(current))
	if !shell.IsWithin(resolved, r.repo) {
		r.Refuse(target + " points at " + current + ", which is not in this checkout — left alone")
		return false
	}
	return r.apply(change{
		would: "would remove " + target + " -> " + current,
		did:   "removed  " + target,
		write: func() string {
			if err := r.tree.remove(target); err != nil {
				return "could not remove " + target + ": " + err.Error()
			}
			return ""
		},
	})
}

// Same table, because a second pass re-deriving what to remove goes out of step with what was
// installed. It goes out of step in the direction hardest to spot, leaving things behind and
// reporting ok.

// A mount resolving into another checkout is outside this run's reach to delete, as it is outside
// its reach to repoint. UnmountTarget refuses it by removing a target only when the target resolves
// under the checkout.

// Unmount is the inverse of Mount, over the same table the caller declared.
func (r *Run) Unmount() {
	r.Say("unmounts")
	for _, mount := range r.configs {
		r.UnmountTarget(mount.Target)
	}
	if len(r.bulk) == 0 {
		return
	}
	r.Say(r.bulkLabel)
	for _, mount := range r.bulk {
		r.UnmountTarget(mount.Target)
	}
}
