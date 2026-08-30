package ecoreport

// The one case here that cannot be written from outside the package. Every other suite in this tool
// drives it through Invocation.Exec over a real repository, which is the right level and is where the
// worktree counting is already covered — but a listing git did not produce cannot be built that way,
// because a real `git worktree list` in a real repo always names at least the worktree it ran in.
//
// That unanswerable case is the one worth pinning: `discard` drops the `.idsd/` exclusion out of
// `.git/info/exclude`, which is shared across every worktree, and it drops it whenever the count comes
// back below 2. A count of 0 read as an answer put a parallel throwaway ship's scratch back in front of
// the next `git add -A` there.

import "testing"

func TestAWorktreeListingIsOnlyAnAnswerWhenItNamesAWorktree(t *testing.T) {
	for _, c := range []struct {
		name      string
		listing   string
		want      int
		wantValid bool
	}{
		{
			name:      "an ordinary repository names the one it ran in",
			listing:   "worktree /repo\nHEAD abc123\nbranch refs/heads/main\n",
			want:      1,
			wantValid: true,
		},
		{
			name:      "a second worktree is counted, and it is what keeps the shared exclusion",
			listing:   "worktree /repo\nHEAD abc\nbranch refs/heads/main\n\nworktree /repo-two\nHEAD def\ndetached\n",
			want:      2,
			wantValid: true,
		},
		// git exiting 0 with nothing this can read. Counted as 0 it is below 2, which is the same branch
		// a single-worktree repo takes — so the exclusion went on the strength of a listing that named
		// nothing at all.
		{
			name:      "an empty listing names no worktree, so it is no answer",
			listing:   "",
			wantValid: false,
		},
		{
			name:      "output in a shape this cannot read is no answer either",
			listing:   "fatal: not a git repository\n",
			wantValid: false,
		},
		// The prefix carries its trailing space: `worktreeconfig` and a path merely mentioning the word
		// are not worktree lines, and counting them inflates the count into the keep-the-exclusion branch
		// for the wrong reason.
		{
			name:      "a line that merely starts with the word is not a worktree line",
			listing:   "worktreeconfig true\nbare\n",
			wantValid: false,
		},
	} {
		t.Run(c.name, func(t *testing.T) {
			got, valid := worktreesIn(c.listing)
			if valid != c.wantValid {
				t.Fatalf("worktreesIn(%q) valid = %v, want %v", c.listing, valid, c.wantValid)
			}
			if valid && got != c.want {
				t.Errorf("worktreesIn(%q) = %d, want %d", c.listing, got, c.want)
			}
		})
	}
}
