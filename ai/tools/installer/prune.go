package installer

import (
	"os"
	"strings"

	"configs/ai/tools/shell"
)

// unmountScan is one directory of mounts and the source root a mount of this checkout's comes from.
type unmountScan struct {
	directory  string
	sourceRoot string
}

// AddUnmountScan declares a directory Mount sweeps for links whose source under sourceRoot is gone.
//
// A skill renamed or deleted takes its source directory with it, and nothing in the mount table names
// the old target any more — so link never sees it, and the link left under `~/.claude/skills/`
// resolves into a directory no checkout has. Only the run that would have written it can notice.
//
// It is a deletion, in a package whose contract is that it refuses rather than deletes, so the
// signature is narrow enough that only a mount of this checkout's own matches: a symlink, whose value
// is absolute, whose parent directory is the source root the caller named. Everything else is left
// where it is, dangling or not, which is why the summary below claims only what it checked. The
// signature cannot ask who wrote the link: ai/README.md documents a by-hand install loop whose links
// are identical to the ones written here.
func (r *Run) AddUnmountScan(directory, sourceRoot string) {
	r.scans = append(r.scans, unmountScan{directory: directory, sourceRoot: sourceRoot})
}

func (r *Run) pruneStaleMounts() {
	if len(r.scans) == 0 {
		return
	}
	r.Say("stale mounts")
	for _, scan := range r.scans {
		r.unmountStale(scan)
	}
}

// A root this checkout cannot read stops the scan before the loop. The loop would remove nothing there
// in any case; what it would print is a clean bill of health over a directory the run never opened.
func (r *Run) unmountStale(scan unmountScan) {
	rootReal := realDir(scan.sourceRoot)
	if rootReal == "" {
		r.Say("  " + scan.sourceRoot + " cannot be read, so no mount under " + scan.directory + " was checked")
		return
	}
	// A root that resolves and holds nothing is the same hazard one step further in, and the one the
	// loop can act on: every mount of this checkout's dangles at once, so the loop reads the machine's
	// whole set as deleted and takes it. A source root emptied by a half-finished checkout is not a set
	// of deletions anybody made. The caller's own emptiness check cannot stand in for this one — the
	// scan runs inside Mount, so whatever a caller does about an empty root, it does afterwards.
	if !holdsSource(rootReal) {
		r.Say("  " + scan.sourceRoot + " holds no source, so no mount under " + scan.directory + " was checked")
		return
	}
	if !shell.IsDir(scan.directory) {
		r.Say("  ok       nothing is mounted at " + scan.directory)
		return
	}

	entries, err := os.ReadDir(scan.directory)
	if err != nil {
		r.Say("  " + scan.directory + " cannot be read, so no mount under it was checked")
		return
	}
	stale := 0
	for _, entry := range entries {
		// A leading dot is passed over, because the shell glob this was ported from never offered one
		// and a sweep that removes more than the one it replaced is the widening this scan's signature
		// exists to prevent.
		if strings.HasPrefix(entry.Name(), ".") {
			continue
		}
		path := shell.Join(scan.directory, entry.Name())
		if !shell.IsSymlink(path) {
			continue
		}
		value := linkValue(path)
		// Absolute only, for the reason mountForeignRoot gives: a relative value resolves against the
		// link's own directory, so resolving it here would resolve it against the wrong one.
		if !strings.HasPrefix(value, "/") {
			continue
		}
		if realDir(shell.DirName(value)) != rootReal {
			continue
		}
		if shell.PathExists(value) {
			continue
		}
		stale++
		r.apply(change{
			would: "would remove " + path + ", which points at " + value + " and nothing is there",
			did:   "removed  " + path + ", which pointed at " + value + " and nothing is there",
			write: func() string {
				if err := r.tree.remove(path); err != nil {
					return "could not remove " + path + ", which points at " + value +
						" and nothing is there: " + err.Error()
				}
				return ""
			},
		})
	}
	if stale == 0 {
		r.Say("  ok       every mount under " + scan.directory + " this checkout wrote still resolves")
	}
}

// Whether a source root holds anything a mount could come from. One directory is enough: the question
// is whether the checkout is half-finished, not how much of it arrived.
func holdsSource(root string) bool {
	entries, err := os.ReadDir(root)
	if err != nil {
		return false
	}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".") {
			continue
		}
		if shell.IsDir(shell.Join(root, entry.Name())) {
			return true
		}
	}
	return false
}
