# Overview and installation

- [`env/`](env/README.md) installs zsh, git, ghostty, neovim, starship and their tools through `env/bootstrap.sh`.
- [`ai/`](ai/README.md) installs agent instructions, kk-flavor standards, skills, Go tools and MCP servers.
  For a project, run only `ai/install-project.sh --agent=claude|codex <project>`.
  Use `ai/bootstrap.sh --agent=claude|codex` only for a requested machine-wide install.
  Both AI installers require `--agent=claude` or `--agent=codex` and support `--uninstall`.

The environment and AI installations are independent. Run either or both, in any order; removing one
leaves the other working. Their READMEs describe what gets installed and how to remove it.

The installers share `lib/mount.sh`. They report and skip targets they don't own, and refuse to move
mounts from another checkout unless you pass `--relocate`. Use `--dry-run` to preview changes.
Keep the checkout: deleting it breaks its symlinks. AI bootstrap also removes stale skill links
when their source disappears from this checkout.

## Tests

`ai/run-tests.sh` discovers every `*-test.sh` in the repository and fails if it finds none.
GitHub Actions runs it on Linux and macOS for pushes to `main` and pull requests, alongside the
other gates in `.github/workflows/gates.yml`.
