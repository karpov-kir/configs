# <Project> Constitution

Long-term memory for IDSD. Read by `idsd-build` as Context. Lean — link out, don't restate code style.

## References

- Code style & conventions: [CLAUDE.md](../CLAUDE.md), [PROJECT_CODE_STYLE.md](../PROJECT_CODE_STYLE.md)

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
