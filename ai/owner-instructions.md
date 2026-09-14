### KK Flavor

Read `~/.kk-flavor/inject.md` now and follow it — applies to all work, skill-invoked or ad-hoc.

## RTK

Use `rtk` for supported shell commands and `rtk proxy <command>` for unsupported commands or exact output. Always use `rtk proxy` when reading a diff for review; compressed output can alter hunk text. Where installed, the command-rewriting hook handles the prefix; otherwise select RTK commands explicitly.

## Worktrees

Create a worktree with `git worktree add ~/Documents/WP/worktrees/<repo-key>/<worktree>`, then enter it by path. `<repo-key>` is what `~/.kk-flavor/scripts/repo-key.sh` prints for the clone. Never make one through a client's own worktree feature: those pick their own location, usually inside the checkout. One already sitting elsewhere stays there.

## Memory

Read `~/Document/AI/MEMORY.md` at the start of every session. Write owner memory only to that file, shared by all owner clients; never to these instructions or a client's automatic memory directory. Preserve existing entries when updating it.
