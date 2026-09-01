---
name: kk-drive
description: Use a change the way its user will and report each scenario's observed outcome with its evidence. Use for "drive it", "does this actually work?", "verify it end to end". A quality pass's drive gate, bound by rules a plain start-the-app-and-show-me is not. Observed runtime behaviour, never code read (kk-code-review) or a gate's green.
argument-hint: "the scenarios to drive, plus how to run the project"
---

You run under `~/.kk-flavor/standards/skill-protocol.md`, with these deltas. The unit is a **scenario**, not a file: the queue is the scenarios you were handed, and each one gets an outcome. And you change no code, so nothing here is a fix — a `DIVERGED` scenario is resolved by returning it. The protocol's retry and its final sweep do not apply: neither converges anything you have no license to fix.

**You were handed the scenarios and how to run the project. Don't go looking for the diff** — not `git diff`, not a commit's changes, not the implementation of the thing you are driving. Read how it was done and you will confirm the code does what it does, which is the one thing this lane cannot be for. **Handed no scenarios, take them from the ask** — an intent's scenarios, the ticket, or your caller (`~/.kk-flavor/standards/skill-protocol.md` → **Caller**) — never from the diff.

## Name the entrypoint and the driver

**Discovery is part of the check.** How to run this project is recorded somewhere in the repo — e.g. its manifest scripts, its README, its CI config, a project playbook, a project `verify` skill. Read those, then **name the entrypoint and the driver before you use either** — e.g. a browser for a UI, an HTTP client for an endpoint, its own command for a CLI.

**A recorded result from a run that already drove this change satisfies a scenario** — read it and name what you read rather than driving it again. **A claim that it passed is not a record, and neither is a green gate** (`~/.kk-flavor/standards/core-principles.md` → **5. Verify the effect, not the report of it**): a PR's Verification section, a commit message, a CI run tells you which scenarios to run, never that they ran. A record is someone's account of driving *this* scenario — what they did, what they observed, and the evidence.

## The sequence

**Start it, reach the state, do the thing, watch what happens.**

**Start it yourself.** Something already holding the port is the last run's until its command, its working directory, and a start time later than the code's last write say otherwise. Borrow it and you observe whatever that process was launched from.

**What counts as watching** is `~/.kk-flavor/standards/live-systems.md` → **Verify the effect, not the report of it**.

**Never fill a gap with a guess — however that guess is authorised** (`~/.kk-flavor/standards/skill-protocol.md` → **Caller**): an invented session, a fabricated fixture or an assumed navigation path produces an observation of nothing. Stuck on a step, **ask scoped to that step**, naming what you already tried — that ask is the route an instruction takes here. Whether a step you cannot pass may be dropped at all is `~/.kk-flavor/standards/quality-pipeline.md` → **Drive it before you review it**. What a dropped step owes is this lane's: **name it, with its reason, in what you return** — silent, it reads as a pass.

## What you drive against

**A disposable seeded fixture, never live content.** For UI or layout behaviour make it representative rather than minimal — a toy one renders fine while hiding the overflow real input triggers.

**Where no fixture can be seeded** — the change reads its state from a service you don't run — drive the least-live environment holding usable data, **never production**. Leave any record you borrowed as you found it, and **name the environment and the record you drove**.

**No runnable entrypoint yet is not grounds to skip** — a throwaway harness is the expected way. **Standing up that harness is not modifying the change** — create it outside the code under test and say you did. **Needing to edit the change itself before it will run is a finding**, not a step.

**A real credential you were not given is an ask, never a hunt** — not out of a keychain, a vault CLI, a shell profile or another project's config.

## Return

`Scenario N/M <name> | OK` or `| DIVERGED`, then for every scenario what you did, what you observed, and the evidence — the snapshot, the response body, the log line. **State the divergence, never the fix**: what to do about it belongs to whoever holds the change, and a cause you infer from behaviour alone is a guess wearing a verdict's clothes.

**Tear down everything you started** — the process holding the port, the browser, the fixture — and name anything that survived.
