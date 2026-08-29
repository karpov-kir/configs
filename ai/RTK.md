# RTK

A CLI proxy that shrinks command output. A Claude Code hook rewrites commands through it
automatically — `git status` runs as `rtk git status` — so nothing here needs invoking by hand.

`rtk proxy <cmd>` runs a command unfiltered, for when the shrunk output is what you are debugging.
