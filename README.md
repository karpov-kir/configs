# Overview & installation

The shell and editor half of this machine, and the agent half, on a machine that may want only one of
them.

- [`env/`](env/README.md) — zsh, git, ghostty, neovim, starship, and the tools they need. Installed by
  `env/bootstrap.sh`.
- [`ai/`](ai/README.md) — Claude Code's instructions, the kk-flavor standards, the skills, the Go tools
  those skills run, and the MCP servers. Installed by `ai/bootstrap.sh`.

Run either, or both, in any order. Neither reads the other's mounts, so a machine can hold one of them
without the other, and removing one leaves the other working. Each half's README lists what its
bootstrap puts in your home, which is also what you delete to remove it again.

Both scripts share `lib/mount.sh`, which is what makes them behave alike. A target you didn't link
yourself is reported and skipped rather than replaced. A machine already mounted from another checkout
is refused before anything is written: run either script from a scratch clone and it stops there,
because otherwise every link would swing over to the clone and deleting the clone would leave you with
no config at all. `--relocate` is how you say you mean it, and `--dry-run` prints what a run would
change without changing it.

## Tests

`ai/run-tests.sh` runs every `*-test.sh` in the repository. It finds them rather than listing them, so
a suite is covered the day it's written, and it exits non-zero if it finds none. GitHub Actions runs it
on pushes to `main` and on pull requests, over Linux and macOS, alongside the repo's other gates in
`.github/workflows/gates.yml`.
