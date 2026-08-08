# Touching a Running System

This covers a deploy, live data, an external write API, an infrastructure change. Core Principle 5's first trap — prove the check can fail — binds everywhere, so it stays in [core-principles.md](core-principles.md); the traps below fire only here.

## Verify the effect, not the report of it

- **Check identity, not liveness.** A healthy response proves the service is up, never which artifact it runs.
- **A setting accepted is not a mechanism engaged.** Read the mechanism's own counter.
- **A feasibility probe must contend for the contested resource**, not a free stand-in.

## Arrange the undo before the act

Capture the current state and name the one-command revert first. Read the repo's own runbook before improvising a path. Never learn an undocumented write API by sending it a payload — read the routes first.
