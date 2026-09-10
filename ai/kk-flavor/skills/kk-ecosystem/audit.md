# Full ecosystem audit

Use only for an explicitly requested whole-ecosystem audit. Resolve the selected client's instruction root and hold the full set under `~/.kk-flavor/standards/ecosystem.md`.

1. Run `~/.kk-flavor/skills/kk-ecosystem/scripts/check.sh --agent="${ECO_AGENT:?choose claude or codex explicitly}"` over the root. Read its exclusions before interpreting the totals.
2. Audit global and project instructions, `~/.kk-flavor/inject.md`, its always-read standards, their explicit imports and loaded skill descriptions. Read each once; revisit changed inputs and affected consumers. Report installed imports outside the writable scope.
3. Record the always-loaded budget before editing. Move instructions needed only on a branch to that branch. Preserve constraints the common path needs.
4. Run `~/.kk-flavor/skills/kk-ecosystem/scripts/ruleecho.sh <root>` and `~/.kk-flavor/skills/kk-ecosystem/scripts/cite-graph.sh <root>`. Inspect semantic contradictions and missing reachability beyond what their textual scans detect.
5. Apply scoped rule, shape and prose checks in that order under `~/.kk-flavor/standards/skill-protocol.md`. Disjoint scopes may use bounded workers; the coordinator reconciles cross-scope rules before final wording. Do not add a relay agent for each check.
6. Re-run wiring and record the resulting budget. Return changed obligations, their remaining homes, uncovered consumers and code handoffs. External instruction-deletion judging is available for disputed cuts; it is not required for every file.
