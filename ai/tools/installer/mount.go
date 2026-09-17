package installer

import (
	"fmt"
	"strings"

	"kk-flavor/tools/shell"
)

// Two lists rather than one, because the guard reports them differently. A config is individually
// consequential — a shell, a git identity, the instructions every agent session loads — so it is
// named. A bulk set is homogeneous, so its count says all a list would.
func (r *Run) AddConfig(source, target string) {
	r.configs = append(r.configs, Mount{Source: source, Target: target})
}

func (r *Run) AddBulk(source, target string) {
	r.bulk = append(r.bulk, Mount{Source: source, Target: target})
}

func (r *Run) BulkMounts() []Mount {
	return r.bulk
}

// RewriteBulkSources replaces every declared bulk source with what rewrite answers for it. The project
// installer is the caller: it declares its skills against this checkout so the second-checkout guard
// has a checkout to recognise, then points them at the shared bucket so a project holds the one path
// every install of this flavor has. Declaring them at the bucket in the first place would leave the
// guard nothing to see and a project quietly repointed off somebody else's clone.
func (r *Run) RewriteBulkSources(rewrite func(source string) string) {
	for i := range r.bulk {
		r.bulk[i].Source = rewrite(r.bulk[i].Source)
	}
}

// Link writes one mount at once, and reports which of the four states it found. The only state that
// writes over something is a symlink, which carries no data of its own — but see the foreign-root
// guard below, which is the case where that reasoning holds for the link and not for what the link is
// part of.
//
// Exported for one caller and one shape: the project installer mounts a bucket that every other mount
// in its table then resolves THROUGH, so that one has to land before the rest are even spelled. Mount
// is still what writes the table, and it links this one again, reporting it as already ok.
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
	// shell.DirName rather than a trim of the last component: a target ending in a slash names itself
	// as its own parent, and a parent created there is a directory the link then lands inside. An empty
	// $HOME leaves a root-level target, whose parent is `/` and not nothing — a run that computed
	// nothing would refuse naming a parent it cannot print, sending the reader after a directory that
	// was never the problem.
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

// link above treats an existing symlink as safe to write over, on the grounds that a symlink carries
// no data of its own. True of the link. The damage this guards is to the MOUNT: when the checkout a
// live link names is real, and is where this machine's config actually lives, that link is not stale —
// this checkout is the stranger, and repointing it moves the human's whole setup here. Delete the
// clone afterwards, which is the entire point of a scratch clone, and their next login has no .zshrc,
// no .gitconfig and no agent instructions. The code whose contract is that it refuses rather than
// deletes would have done it by reporting "repointed" thirty-odd times.
//
// Narrow on purpose, so the rule link states keeps working. A mount counts as belonging to another
// checkout on two conditions: its link value ends in the same relative path — `…/zsh/.zshrc` for
// `~/.zshrc` — and the root left over when that path is stripped holds a copy of the calling
// installer. Together those make it the same file in a second copy of this repository, rather than an
// unrelated config the stale-symlink rule is right to repoint. A README-era link with a trailing
// slash still compares equal to this checkout and stays the compatibility case it was.
//
// A root that no longer resolves does not count either. That is the aftermath of this very bug, or of
// a checkout moved on purpose, and repointing a dangling link is the repair rather than the damage.
func (r *Run) mountForeignRoot(source, target string) string {
	if !shell.IsSymlink(target) {
		return ""
	}
	current := linkValue(target)
	// Absolute only. A relative link value resolves against the link's own directory, not this
	// process's working directory, so naming a root from it would name the wrong one. Every link
	// written here is absolute, so a relative one was not written here and is not one of these mounts.
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
	// And the root has to hold a copy of the calling installer, not merely end in a matching path
	// component. Several sources are one component long — `nvim`, `ghostty`, `kk-flavor` — so the tail
	// comparison alone reads `~/.config/nvim -> ~/.dotfiles/nvim` as a second checkout, refuses the
	// whole run, and tells the human their configuration is mounted from a directory that has never
	// held it. That link is an ordinary stale mount and link is right to repoint it.
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

// How many mounts a root holds, spelled the way the guard reports it. With a bulk set the count leads
// and the kinds are broken out; without one there is nothing to break out.
func (r *Run) mountTally(named, bulk int) string {
	if bulk > 0 {
		return fmt.Sprintf("%d mounts (%d configs and %d %s)", named+bulk, named, bulk, r.bulkLabel)
	}
	return fmt.Sprintf("%d mounts", named+bulk)
}

// One root's worth of the refusal: what resolves to it, what running from here would do to them, and
// the refusal line the exit code carries. The count leads, before any list. Every line of the list
// carries the same root, so the list is one fact repeated; the scale is the fact a reader cannot
// reconstruct, and a reader who takes in the named configs and stops has not learned that every one of
// their skills moves too.
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

// Mount surveys every mount, refuses if this machine's config lives in another checkout, then links.
// One call rather than two, because the survey has to cover every mount before the first is written
// and a caller able to skip it is a caller able to skip the guard.
//
// It answers false when the guard stopped it before the first write, which is the caller's cue to
// report and exit rather than carry on into the steps that follow mounting.
func (r *Run) Mount() bool {
	r.Say("mounts")
	foreignTotal := r.surveyForeignMounts()

	switch {
	case foreignTotal == 0:
		// Said out loud on the way past. A guard that prints nothing when it passes reads exactly like
		// a guard that was never reached, and this one runs on every machine that is already set up.
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

	// Inside this function rather than beside it, so a caller cannot skip the scan and keep a mount
	// nothing can reach; after the links, because a machine mounted from another checkout writes
	// nothing at all, removals included.
	//
	// pruneStaleMounts, NOT Unmount: this drops mounts whose SOURCE is gone, while Unmount removes
	// every mount the run declared. They arrived from two branches under one name, and a normal install
	// calling the second links the whole tree and then tears it down.
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

// UnmountTarget removes one mount, and only when this checkout can prove it wrote it. Named by the
// caller rather than taken off the table, because one caller has a target the table never held: the
// Codex install migrates a skill out of an older mount directory once its replacement exists.
//
// The proof is the whole of why this lives here rather than in a caller's own removal: a target is
// removed only if it is a symlink AND its value resolves under the checkout. Anything else — a real
// file, a symlink into somebody else's checkout, a link that resolves nowhere — is reported and left,
// which is the same promise link makes at the other end. ai/README.md performed this proof by hand
// with a `readlink` case matching a path fragment; against the checkout it cannot mistake a
// similarly-named directory for this one.
//
// A target that is already gone is success, not a refusal. Uninstall run twice is ordinary.
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
	// Absolute only, the same test mountForeignRoot makes and for the same reason: a relative value
	// resolves against THIS process's working directory, not the link's own, so ownership would be
	// judged from somewhere the link never named — and a link reading `notmine` would resolve under the
	// checkout and be deleted. Every link written here is absolute, so a relative one was not written
	// here and is not this run's to remove.
	if !strings.HasPrefix(current, "/") {
		r.Refuse(target + " points at the relative path " + current + ", which this never writes — left alone")
		return false
	}
	// Resolved rather than string-compared, so a link written through a differently-spelled but
	// equivalent path is still recognised as this checkout's. A link resolving nowhere resolves to
	// empty and falls through to the refusal, which is right: its target is unknown, so its ownership
	// is too.
	resolved := realDir(shell.DirName(current))
	if resolved == "" || (resolved != r.repo && !strings.HasPrefix(resolved, r.repo+"/")) {
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

// Unmount is the inverse of Mount, over the same table the caller declared. Same table because a
// second pass re-deriving what to remove drifts from what was installed, and drifts silently in the
// one direction nobody notices: leaving things behind and reporting ok.
//
// A mount resolving into another checkout is not this run's to delete any more than it is this run's
// to repoint. UnmountTarget is what refuses it, by removing a target only when the target resolves
// under the checkout.
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
