### KK Flavor

Read `~/.kk-flavor/inject.md` now and follow it — applies to all work, skill-invoked or ad-hoc.

## RTK

Use `rtk` for supported shell commands and `rtk proxy <command>` for unsupported commands or exact output. Always use `rtk proxy` when reading a diff for review; compressed output can alter hunk text. Where installed, the command-rewriting hook handles the prefix; otherwise select RTK commands explicitly.

## Worktrees

Create a worktree with `git worktree add ~/Documents/WP/worktrees/<repo-key>/<worktree>`, then enter it by path. `<repo-key>` is what `~/.kk-flavor/scripts/repo-key.sh` prints for the clone. Never make one through a client's own worktree feature: those pick their own location, usually inside the checkout. One already sitting elsewhere stays there.

## Tooling

Install tooling through mise, Docker, brew on a Mac or choco on Windows. Never use a vendor installer or install anything ad hoc into the system. Ask the owner before you install anything.

## Memory

Owner memory is a record that all owner clients share in `~/Documents/AI/MEMORY.md`. Write owner memory only to that file. Never write it to these instructions or to a client's automatic memory directory. Keep its existing entries when you update it. No session or worker reads it at start. Its entries reach agents when they are promoted into the standard or instruction that owns their lane. That integration runs from time to time, under `~/.kk-flavor/standards/records.md` → **Promotion is the exit upward**.
