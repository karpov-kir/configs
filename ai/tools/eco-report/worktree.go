package ecoreport

import (
	"crypto/rand"
	"encoding/hex"
	"os"

	"kk-flavor/tools/shell"
)

// Which worktree this is, what the report records about the one that reviewed it, and whether the two
// agree. The tree fingerprint cannot answer any of it: two clean worktrees of one clone at the same
// commit fingerprint identically, and the scratch directory holding the stamp is now shared per clone
// — so without an identity a sibling reads a review it never earned as its own.
//
// One decision serves both readers: `gate` explains which way it went, `state` routes on it. Split them
// and they drift. That already happened once — gate grew a guard for a stamped tree with no worktree
// line, state did not, and one report read as reviewed to gate and unreviewed to state.

// What the report's `reviewed-worktree` line says about this worktree.
type worktreeVouch int

const (
	// The recorded token is this worktree's own.
	vouchesForThisWorktree worktreeVouch = iota
	// A stamped tree with no usable worktree beside it. Reports written before this field existed look
	// like that, and so does one a human copied out of an in-tree .idsd/ by hand, which
	// reconcileTreeIdsdDir tells them to do. Without this case the missing line reads back as "", which
	// isUnstamped accepts, and every worktree gates clean off that stamp.
	noReviewingWorktreeRecorded
	// No stamp stands here at all, so there is no review to vouch for anything yet.
	reviewNotYetStamped
	// This worktree has no identity to compare, which must never read as a match. `worktreeToken` says
	// what goes wrong when it does.
	thisWorktreeHasNoIdentity
	// A real token, and not this one's.
	reviewedInAnotherWorktree
)

// The verdict, and this worktree's own token where one could be established — empty otherwise, since
// "could not be established" is not a value anything may compare.
func (r *run) worktreeVouch() (worktreeVouch, string) {
	recorded := r.reviewedWorktreeToken()
	switch {
	case !isWorktreeToken(recorded) && !isUnstamped(r.reviewedTree()):
		return noReviewingWorktreeRecorded, ""
	case isUnstamped(recorded):
		return reviewNotYetStamped, ""
	}
	mine, established := r.worktreeToken()
	switch {
	case !established:
		return thisWorktreeHasNoIdentity, ""
	case recorded != mine:
		return reviewedInAnotherWorktree, mine
	}
	return vouchesForThisWorktree, mine
}

// Which worktree's tree the stamp was taken in, as `<token> <path>`. The token is what gets compared;
// the path is there for the human, because a path is NOT an identity. Compare paths and it breaks both
// ways. `git worktree remove` then `git worktree add` at the same path gives a brand-new worktree that
// has run nothing a clean gate off its predecessor's stamp, and recreating a scratch worktree at a
// reused path is ordinary practice. `git worktree move` tells the worktree that DID the work that it
// did not.
func (r *run) reviewedWorktreeToken() string {
	return firstField(fieldValue(r.report, "reviewed-worktree"))
}

// This worktree's identity, as the `<token> <path>` pair the stamp records — or false when no identity
// could be established, which the caller must refuse on rather than record.
//
// The path is collapsed to one line because frontmatter is single-line and this half of the pair is not
// a value this tool chose: `git rev-parse --show-toplevel` hands back a control byte in a checkout's
// path verbatim. A newline here writes a second frontmatter line, and since rewriteStamp emits
// `reviewed-worktree:` ABOVE `reviewed-stages:` while every reader takes the first line matching a
// prefix, a forged stage record injected here is the one the merge gate reads. An ESC or CR instead
// rewrites the gate's own BLOCK line on the terminal.
func (r *run) currentWorktreeRecord() (string, bool) {
	token, ok := r.worktreeToken()
	if !ok {
		return "", false
	}
	return token + " " + shell.Oneline(r.currentWorktreePath()), true
}

// The path, for messages only — never for the comparison.
func (r *run) currentWorktreePath() string {
	if real := shell.CanonicalDir(r.root); real != "" {
		return real
	}
	return r.root
}

// A token naming THIS worktree, minted once and kept in the worktree's own private git dir. That
// location is the whole mechanism: git deletes it with `worktree remove`, so a worktree recreated at
// the same path mints a new one and correctly reads as different; and git keeps it across
// `worktree move`, so a relocated worktree keeps its identity.
//
// The bool means "an identity was established". It is NOT a token value: return a sentinel string like
// `unmintable` on failure and it compares equal to itself, so two worktrees that both failed to mint
// gate clean off each other's review. Nor is that pairing a coincidence rare enough to wave off:
// minting fails when the private git dir cannot be written, which is a property of the clone, so it
// tends to fail in every worktree at once.
func (r *run) worktreeToken() (string, bool) {
	path := r.gitPath("idsd-worktree-id")
	if content, err := os.ReadFile(path); err == nil {
		if token := firstField(string(content)); isWorktreeToken(token) {
			return token, true
		}
	}
	raw := make([]byte, worktreeTokenBytes)
	if _, err := rand.Read(raw); err != nil {
		return "", false
	}
	token := hex.EncodeToString(raw)
	// 0700/0600, not the umask's answer, for a sharper reason than the stage markers' — this file is the
	// only SECRET in the tool. The merge gate compares the recorded `reviewed-worktree` token against it
	// to tell this worktree's review from a sibling's, so any account that can read the token can write
	// it into a report of its own and gate an unqualified tree clean. Left at the umask's answer it
	// landed 0644 under the ordinary 022, and world-writable under the 002 a shared CI image sets.
	if err := os.MkdirAll(shell.DirName(path), 0o700); err != nil {
		return "", false
	}
	if err := os.WriteFile(path, []byte(token+"\n"), 0o600); err != nil {
		return "", false
	}
	return token, true
}

// How many random bytes a token carries. isWorktreeToken derives its hex length from this, so the
// check cannot drift from what it is checking.
const worktreeTokenBytes = 8

// A well-formed token: exactly the hex this mints. Anything else in the file mints a fresh token and
// reads as a different worktree, which is the safe direction to fail.
func isWorktreeToken(value string) bool {
	if len(value) != worktreeTokenBytes*2 {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}
