package installer

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"kk-flavor/tools/shell"
)

// An idempotent region a run owns inside a file it does not: write it, detect it, remove it, and
// refuse rather than clobber anything else in there.
//
// Why this is not part of the mount table above: that only ever writes symlinks, and only at targets
// it can prove it owns — link refuses the moment a target exists and is not a symlink. Writing bytes
// into a file someone else authored is the exact inverse, so it is held apart rather than widening a
// contract env/bootstrap.sh also depends on.
//
// The fences are the caller's, not this file's. A CLAUDE.md region is fenced with HTML comments so it
// is invisible when the markdown renders; a .gitignore region is fenced with `#` lines. Nothing here
// knows more than that it was handed two marker lines and a body, which is what lets one
// implementation serve both and keeps `.claude`, CLAUDE.md, skills and projects out of its vocabulary.

// regionState is whether the region is there, and whether the file is safe to touch at all.
type regionState int

const (
	// regionAbsent: the file has neither fence — a write appends.
	regionAbsent regionState = iota
	// regionPresent: both fences, in order — a write rewrites between them.
	regionPresent
	// regionConflict: one fence without the other, a close before its open, or a second open.
	//
	// The refuse-rather-than-clobber case, and deliberately loud: half a fence means something edited
	// inside the region or truncated the file, and either way the span a write would rewrite is no
	// longer the span that was written. Guessing at its extent is how an installer eats a paragraph the
	// human wrote.
	regionConflict
)

func readRegionState(lines []string, openFence, closeFence string) regionState {
	seenOpen, seenClose := false, false
	for _, line := range lines {
		if line == openFence {
			// A second open before the close is the same damage as a missing one: two regions, and no
			// way to say which is this run's.
			if seenOpen {
				return regionConflict
			}
			seenOpen = true
			continue
		}
		if line == closeFence {
			if !seenOpen || seenClose {
				return regionConflict
			}
			seenClose = true
		}
	}
	switch {
	case seenOpen && seenClose:
		return regionPresent
	case !seenOpen && !seenClose:
		return regionAbsent
	default:
		return regionConflict
	}
}

// The guard both writers run first. Everything here is a reason to touch nothing, and each one is a
// different sentence because they send a reader somewhere different.
//
// A missing file is refused rather than created: this is for regions inside files that already exist,
// and a caller that wants one created says so itself. Creating it here would let a typo in a path
// produce a plausible-looking new file in someone's repository.
//
// A symlink is refused for the mirror of link's reason — writing through one edits a file in a place
// the caller never named, which for a CLAUDE.md symlinked into a checkout means editing the checkout.
func (r *Run) regionWritable(file string) bool {
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

// The sentence a failed replacement gets. Each names a different step, because a reader sent to check
// whether the original survived has to know whether it could have.
func replaceRefusal(file string, err error) string {
	switch {
	case errors.Is(err, errNoTemporary):
		return "could not create a temporary file beside " + file + " — nothing was written"
	case errors.Is(err, errNotWritten):
		return "could not write the new " + file + " — the original is untouched"
	default:
		return "could not replace " + file + " — the original is untouched"
	}
}

// WriteRegion appends the region when it is absent, rewrites between the fences when it is present,
// and says nothing changed when what is there already matches byte for byte.
func (r *Run) WriteRegion(file, openFence, closeFence, body string) bool {
	if !r.regionWritable(file) {
		return false
	}
	content, err := os.ReadFile(file)
	if err != nil {
		r.Refuse("could not read " + file + " — nothing was written: " + err.Error())
		return false
	}
	lines := shell.SplitLines(string(content))

	switch readRegionState(lines, openFence, closeFence) {
	case regionConflict:
		r.Refuse(file + " holds one half of the " + openFence + " region — something edited inside it, so nothing was written")
		return false
	case regionPresent:
		return r.rewriteRegion(file, lines, openFence, closeFence, body)
	default:
		return r.appendRegion(file, content, openFence, closeFence, body)
	}
}

func (r *Run) rewriteRegion(file string, lines []string, openFence, closeFence, body string) bool {
	// Compared against what is between the fences, not against the whole file, so an unrelated edit
	// elsewhere in the human's file never looks like this region drifting.
	if strings.Join(regionBody(lines, openFence, closeFence), "\n") == body {
		r.Say("  ok       " + file + " already carries the " + openFence + " region")
		return true
	}
	return r.apply(change{
		would: "would rewrite the " + openFence + " region in " + file,
		did:   "rewrote  the " + openFence + " region in " + file,
		write: func() string {
			var kept []string
			skipping := false
			for _, line := range lines {
				switch {
				case line == openFence:
					kept = append(kept, line, body)
					skipping = true
				case line == closeFence:
					skipping = false
					kept = append(kept, line)
				case !skipping:
					kept = append(kept, line)
				}
			}
			if err := r.tree.replaceFile(file, joinLines(kept)); err != nil {
				return replaceRefusal(file, err)
			}
			return ""
		},
	})
}

func (r *Run) appendRegion(file string, content []byte, openFence, closeFence, body string) bool {
	return r.apply(change{
		would: "would add the " + openFence + " region to " + file,
		did:   "added    the " + openFence + " region to " + file,
		write: func() string {
			var out strings.Builder
			out.Write(content)
			// A blank line ahead of it when the file does not already end in one, so the region never
			// fuses onto the human's last paragraph. A file not ending in a newline gets one first, or
			// the open fence lands on the end of their final line.
			if len(content) > 0 && content[len(content)-1] != '\n' {
				out.WriteString("\n")
			}
			if len(content) > 0 {
				out.WriteString("\n")
			}
			out.WriteString(openFence + "\n" + body + "\n" + closeFence + "\n")
			if err := r.tree.replaceFile(file, []byte(out.String())); err != nil {
				return replaceRefusal(file, err)
			}
			return ""
		},
	})
}

// RemoveRegion removes the region and nothing else. Absent is success, not a refusal — an uninstall
// run twice is a thing people do, and the second run has nothing to say beyond "already gone".
//
// The blank line the writer added ahead of the region goes with it, so install-then-uninstall leaves
// the file as it was found rather than growing a blank line per cycle.
func (r *Run) RemoveRegion(file, openFence, closeFence string) bool {
	if !r.regionWritable(file) {
		return false
	}
	content, err := os.ReadFile(file)
	if err != nil {
		r.Refuse("could not read " + file + " — nothing was removed: " + err.Error())
		return false
	}
	lines := shell.SplitLines(string(content))

	switch readRegionState(lines, openFence, closeFence) {
	case regionConflict:
		r.Refuse(file + " holds one half of the " + openFence + " region — its extent is not ours to guess, so nothing was removed")
		return false
	case regionAbsent:
		r.Say("  ok       " + file + " carries no " + openFence + " region")
		return true
	}
	return r.apply(change{
		would: "would remove the " + openFence + " region from " + file,
		did:   "removed  the " + openFence + " region from " + file,
		write: func() string {
			if err := r.tree.replaceFile(file, joinLines(withoutRegion(lines, openFence, closeFence))); err != nil {
				return replaceRefusal(file, err)
			}
			return ""
		},
	})
}

func regionBody(lines []string, openFence, closeFence string) []string {
	var body []string
	inside := false
	for _, line := range lines {
		switch {
		case line == closeFence:
			inside = false
		case inside:
			body = append(body, line)
		case line == openFence:
			inside = true
		}
	}
	return body
}

func withoutRegion(lines []string, openFence, closeFence string) []string {
	var kept []string
	inside, holding := false, false
	for _, line := range lines {
		switch {
		case line == openFence:
			inside = true
		case inside && line == closeFence:
			inside = false
		case inside:
			// dropped with the region
		default:
			// One blank line immediately before the open fence is this run's — the writer put it there.
			// Held back rather than emitted, and flushed only if something follows, so the file does not
			// end on it.
			//
			// A flag rather than the blank line itself: a held blank stored as the line is the empty
			// string, which is what "holding nothing" also looks like, so the flush never fires and
			// EVERY blank line in the file goes with the one this run owns.
			if holding {
				kept = append(kept, "")
				holding = false
			}
			if line == "" {
				holding = true
				continue
			}
			kept = append(kept, line)
		}
	}
	return kept
}

// Lines back to bytes, each one newline-terminated. No lines is an empty file rather than a lone
// newline, which is what a file whose every line was the region should come back as.
func joinLines(lines []string) []byte {
	if len(lines) == 0 {
		return nil
	}
	return []byte(strings.Join(lines, "\n") + "\n")
}
