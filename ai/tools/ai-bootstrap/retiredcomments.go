package aibootstrap

import (
	"os"
	"path/filepath"
	"strings"
	"time"
)

// The comment pipeline was retired for comment-pass, which keeps one reply per file and nothing else.
// The pipeline mounted a comment-writer agent and kept archives and caches under the state and cache
// directories. This undoes both on a machine that ran it. The archives move to the Trash, where their
// owner can still restore them, and nothing of them is deleted.

// retiredCommentState is what the retired pipeline kept, relative to the state and cache directories.
var retiredCommentState = []string{
	".local/state/kk-flavor/comment-archive",
	".local/state/kk-flavor/comments",
	".cache/kk-flavor/derived-names",
	".cache/kk-flavor/vocabulary",
	".cache/kk-flavor/writer-eval-slots",
}

// removeRetiredCommentPipeline removes an agent link whose source this checkout no longer ships, and
// moves the retired pipeline's state to the Trash.
func (run *invocation) removeRetiredCommentPipeline() {
	if !run.isOwner {
		return
	}
	var said bool
	say := func(line string) {
		if !said {
			run.mounting.Say("retired comment pipeline")
			said = true
		}
		run.mounting.Say(line)
	}
	agents := filepath.Join(run.Home, ".claude", "agents")
	shipped := filepath.Join(run.Repo, "kk-flavor", "agents") + string(filepath.Separator)
	entries, _ := os.ReadDir(agents)
	for _, entry := range entries {
		link := filepath.Join(agents, entry.Name())
		target, err := os.Readlink(link)
		if err != nil || !strings.HasPrefix(target, shipped) {
			continue
		}
		if _, err := os.Stat(target); err == nil {
			continue
		}
		switch {
		case run.isDryRun:
			say("  would remove " + link + ", whose agent the flavor no longer ships")
		case os.Remove(link) != nil:
			run.mounting.Refuse("could not remove " + link)
		default:
			say("  removed  " + link + ", whose agent the flavor no longer ships")
		}
	}
	trash := filepath.Join(run.Home, ".Trash")
	stamp := time.Now().Format("2006-01-02")
	for _, rel := range retiredCommentState {
		from := filepath.Join(run.Home, rel)
		if _, err := os.Stat(from); err != nil {
			continue
		}
		to := filepath.Join(trash, strings.ReplaceAll(rel, "/", "_")+"-retired-"+stamp)
		switch {
		case run.isDryRun:
			say("  would move " + from + " to the Trash")
		case os.MkdirAll(trash, 0o700) != nil || os.Rename(from, to) != nil:
			run.mounting.Refuse("could not move " + from + " to the Trash")
		default:
			say("  moved    " + from + " to " + to)
		}
	}
}
