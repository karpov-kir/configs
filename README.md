# Overview and installation

- [`env/`](env/README.md) installs zsh, git, ghostty, neovim, starship and their tools through `env/bootstrap.sh`.
- [`ai/`](ai/README.md) installs agent instructions, kk-flavor standards, skills, Go tools and MCP servers.
  For a project, run only `ai/install-project.sh --agent=claude|codex <project>`.
  Use `ai/bootstrap.sh --agent=claude|codex` only for a requested machine-wide install.
  Both AI installers require `--agent=claude` or `--agent=codex` and support `--uninstall`.

The environment and AI installations are independent. Run either or both, in any order; removing one
leaves the other working. Their READMEs describe what gets installed and how to remove it.

The installers share one mounting library, `ai/tools/installer/`. They report and skip targets they
don't own, and refuse to move mounts from another checkout unless you pass `--relocate`. Use `--dry-run` to preview changes.
Keep the checkout: deleting it breaks its symlinks. AI bootstrap also removes stale skill links
when their source disappears from this checkout.

## Tests

`ai/gate.sh` is the whole of it: gofmt, `go vet`, `go test ./...`, the wiring check and the field
guide. It fails a run over 100 seconds, which is the bound
[`testing.md`](ai/kk-flavor/standards/testing.md) rule 6 sets. `--full` defeats Go's own test cache,
and that is what the bound is measured against.

There are no shell suites. The installers and the MCP tools are Go, and every other script here is a
stub that execs a Go binary. Four scripts hold real shell, all of them covered by Go suites inside the
module: `ai/tools/resolve.sh`, `install.sh` and `source-stamp.sh`, which exist to reach a Go binary
from a checkout that has none yet, and `ai/mcp-env.sh`, which an MCP client launches from a path
written into its config.

GitHub Actions runs these checks for pushes to `main` and pull requests, in
`.github/workflows/gates.yml`. It leaves out the field guide and adds an 80% coverage floor.
