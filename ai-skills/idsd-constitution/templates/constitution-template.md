# <Project> Constitution

Long-term memory for IDSD. Read by `idsd-build` as Context. Lean — link out, don't restate code style.

## References

- Code style & conventions: <link the project's `CLAUDE.md` and `PROJECT_CODE_STYLE.md` if they exist; otherwise note that the global `~/.claude/CLAUDE.md` governs — link only to files that actually exist>

## Principles

Project-specific non-negotiables (3–7), beyond general code style:

- <principle>
- <principle>

## Baseline NFRs

Baseline constraints every intent inherits unless its own constraints override:

- Performance: <e.g. p99 < 200ms>
- Accessibility: <e.g. WCAG 2.1 AA>
- Security: <e.g. no secrets in repo, authn on all endpoints>
- Coverage floor: <e.g. ≥ 90%>

## Gate commands

Real, runnable commands `idsd-build` uses to resolve gates:

| Gate | Command |
|------|---------|
| build | `<cmd>` |
| lint | `<cmd>` |
| test | `<cmd>` |
| coverage | `<cmd>` |
| perf | `<cmd>` |
