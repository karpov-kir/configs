---
name: kk-ecosystem
description: Refine agent instructions, reconcile rules and check their wiring. Use for "refine the ecosystem", "de-bloat instructions", or the instruction lane. Scoped repair by default; a whole-ecosystem audit must be requested. Skill triggering and structure alone belong to kk-skillcraft; wording alone to kk-edit.
argument-hint: "the instruction files or change set; say whole ecosystem for the full audit"
audience: maintainer
---

Refine the named instruction scope so it steers agents with less redundant reading. Stop when its rules, structure, prose and affected wiring agree.

Read `~/.kk-flavor/standards/ecosystem.md` for the deletion and ownership rules. Run under `~/.kk-flavor/standards/skill-protocol.md` with the protected `instructions` role from `~/.kk-flavor/standards/model-policy.md`. This worker owns the ordered checks below; they do not create nested agents.

## 1. Resolve and check

Resolve the requested files or diff. If no scope is named, use the current change set; with neither, ask the caller for the target. A full audit is explicit: read `~/.kk-flavor/skills/kk-ecosystem/audit.md`, the whole additional procedure for that path.

Run `~/.kk-flavor/skills/kk-ecosystem/scripts/check.sh --agent="${ECO_AGENT:?choose claude or codex explicitly}"` over the ecosystem root. Repair findings in scope; return findings outside it. Follow inbound references when a change can affect their claims.

## 2. Cut, or move

Reconcile contradictions within changed passages and across their callers. Keep one authoritative home for each rule. Before deletion, apply `~/.kk-flavor/standards/ecosystem.md` → **Move it before you cut it**. Check that consumers can still reach the surviving instruction.

Use `~/.kk-flavor/skills/kk-ecosystem/scripts/ruleecho.sh <root>` when searching for duplicated rules, and `~/.kk-flavor/skills/kk-ecosystem/scripts/cite-graph.sh <root>` when changing ownership or reachability. Their output supports the review; neither proves semantic consistency.

## 3. Shape and prose

Apply `kk-skillcraft` inline to the skill directories and instruction artifacts already held. Then apply `kk-edit` inline to their settled text. Rule decisions precede structure, and structure precedes wording. A caller that explicitly owns either later check receives its handoff instead; never run it twice.

## 4. Account for it

Re-run the wiring check after edits. Return scoped verdicts, changed rule behavior and its authorization, remaining findings, and any code-review handoffs for scripts changed here. Report moved or deleted obligations and where surviving consumers find them. Full audit also reports the always-loaded budget before and after.

## Rules

Relocating or pruning rules against the existing bar is in scope. New behavior is applied only where the caller's requirement authorizes it; otherwise return a proposal. Preserve the meaning of retained constraints during the prose pass.
