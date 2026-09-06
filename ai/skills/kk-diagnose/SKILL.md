---
name: kk-diagnose
description: Find the cause of a bug, a flake or a performance regression. Use for "diagnose", "debug this", "why is this failing", "why is this slow", or any report of something broken. Cause-finding from a running system; reading a diff for bugs is kk-code-review's, and driving a change's stated scenarios is kk-drive's.
argument-hint: "the symptom, in the words of whoever hit it"
---

Find what causes the symptom. **Phase 1 is the skill** — with a loop that goes **red** on this symptom, bisection, probes and theories all just consume it; without one, no amount of reading finds the cause.

**Protocol.** You run under `~/.kk-flavor/standards/skill-protocol.md`, with these deltas. The unit is a **symptom**, not a file — one per run, so the queue, the ledger and the final sweep do not apply. The phases below are the loop, and the safety stop is Phase 1's own exit. This skill quotes commands, output and captured artifacts throughout, so → **Redact before you quote** binds every phase.

**You are the diagnosis lane** (`~/.kk-flavor/standards/quality-pipeline.md` → **The round**).

## 1. Build a loop that goes red

**The command comes before the theory.** Phase 2 opens once you can name **one command you have already run**, showing the invocation and its output, that is:

- **Red-capable** — it drives the code path and asserts the **symptom the reporter named**, so it reddens now and greens once fixed. Running without erroring is a different claim.
- **Deterministic** — same verdict every run.
- **Fast** — the bar is `~/.kk-flavor/standards/testing.md` → **1. Core philosophy**, rule 6, and it binds a diagnosis loop hardest, because you will run this one hundreds of times.
- **Agent-runnable** — you can run it unattended.

Spend disproportionate effort here. Try these shapes, roughly in this order: a failing test at the nearest seam; a request against a running instance; a command-line invocation diffed against known-good output; a headless browser script asserting on DOM, console or network; a captured payload replayed through the path in isolation; a throwaway harness standing up the smallest subset that reaches the code; a property or fuzz loop where the symptom is *sometimes wrong*; a bisection harness where it appeared between two known states; a differential run of two versions or configs, diffed.

Then **tighten** what you have: cache the setup, narrow the scope, pin the clock, seed the randomness, isolate the filesystem.

**Where the symptom is intermittent, the target is a higher reproduction rate, not a clean repro** — loop the trigger, parallelise it, add load, widen or narrow the timing window. Half the runs is diagnosable; one in a hundred is not.

**No loop is a stop, and you say so.** Name what you tried, then ask for the environment that reproduces it, a redacted artifact from it, or permission to instrument the system it runs on (`~/.kk-flavor/standards/skill-protocol.md` → **Caller**).

## 2. Reproduce, then minimise

Run it and watch it redden. Confirm it reddens on the **reporter's** symptom rather than a neighbouring failure, and capture the exact symptom — the message, the wrong value, the timing — so the later phases have something to compare against.

Then shrink it: cut inputs, callers, config, data and steps **one at a time**, re-running after each cut. Done when every remaining element is load-bearing — removing any one of them turns the loop green.

## 3. Rank falsifiable hypotheses

Write **3 to 5** before testing any of them; generating one anchors the whole run on whichever idea arrived first.

Each states its prediction — *if X is the cause, then changing Y makes the symptom go, and changing Z makes it worse*. One you cannot phrase that way is a hunch: sharpen it or drop it.

**Put the ranked list to your caller before you test it**, and carry on with your own ranking rather than waiting (`~/.kk-flavor/standards/skill-protocol.md` → **Orchestrators — interactive first**). They often re-rank it in a sentence, or name one already ruled out.

## 4. Instrument one variable at a time

Each probe maps to one prediction from Phase 3. Reach for the debugger or a REPL where the runtime has one — a single breakpoint outreads ten log lines — then targeted logs at the boundary that separates two hypotheses.

**Tag every probe with a unique prefix** — `[DEBUG-a4f2]` — so Phase 6 removes them with one grep. An untagged probe is the one that ships.

**For a regression in speed, measure first.** Establish a baseline with a timing harness, a profiler or the query plan, then bisect against it; logs answer *what ran*, never *what cost*.

What counts as a reading here is `~/.kk-flavor/standards/core-principles.md` → **5. Verify the effect, not the report of it**.

**Phase 5 opens once a probe has confirmed one prediction and falsified the rest.** A hypothesis left merely unrefuted is untested, and a probe that moves two of them at once separates nothing.

## 5. Fix, behind a regression test

Write the test first, at a seam that exercises the pattern **as it occurs at the call site** (`~/.kk-flavor/standards/testing.md` → **1. Core philosophy**). A seam too shallow to hold the real pattern — one caller where the symptom needs several, a unit that cannot reproduce the chain — buys false confidence.

**Where no seam reaches the pattern, that absence is itself a finding**: name it and hand it to the refactor lane (`~/.kk-flavor/standards/skill-protocol.md` → **Finish in the lanes your edits opened**). The architecture is what is keeping the symptom from being locked down.

Otherwise: turn the minimised repro into a test, watch it fail, apply the fix, watch it pass, then re-run Phase 1's loop against the **original** un-minimised scenario. Your fix is unreviewed code (`~/.kk-flavor/standards/skill-protocol.md` → **Your own fixes are unreviewed code**).

## 6. Close out

Every line below is checked before you report a cause:

- The original scenario no longer reproduces — Phase 1's loop, re-run and shown.
- The regression test passes, or the missing seam is named and routed.
- Every tagged probe is gone — grep the prefix and show the empty result.
- Throwaway harnesses are deleted, or moved somewhere that names them as such.

## Return

`Symptom <name> | CAUSE FOUND` or `| NO LOOP`, then the loop's command and the evidence it reddened, the hypothesis that held and what falsified the others, the fix, and the regression test or the seam that is missing.

**State the cause you demonstrated, and label separately anything you only infer.** A cause read out of the code and never driven through the loop is a theory, whatever the fix that followed it did.
