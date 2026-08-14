# Agent brief

`SKILL.md` hands this file verbatim to every Phase 3–5 agent, alongside that agent's scope.

---

You are one scoped agent in a campaign shrinking an ecosystem of agent instructions. **You apply edits directly** within your scope.

## Read before you edit

1. `~/.kk-flavor/standards/ecosystem.md` — the bar. Every judgment you make is this file's, not your taste.
2. `~/.claude/skills/kk-ecosystem/SKILL.md` — the pass you are running over your scope.
3. The adjudicated plan your caller names — **whole**, not only your own section.

## The plan's authority

- **Accepted** entries for your files: apply them.
- **Modified** entries override the raw cut list; their numbers were verified against real files and the cut list's were not.
- **Rescued** entries are binding. The passage must survive in the file named — tighten its wording, never remove its instruction. Disagree → escalate, do not act.
- **Anything the plan did not consider is still yours to cut.**

## The one failure mode that matters

**De-duplication to zero**: two files each state a rule, each copy is deleted citing the other as its home, and the rule now exists nowhere.

Before deleting anything because "another file covers it", **open that file at the moment you delete — not earlier — confirm the text is there, and confirm the plan does not also schedule it.** A concurrent agent may be deleting its copy while you read yours. If both copies are scheduled, keep one and say which.

## Move before you cut

Prefer a move (`~/.kk-flavor/standards/ecosystem.md` → **Move it before you cut it**) to a deletion
you would have to argue for.

**Extracting a rule into a script owes two things.** The case in the `<name>-test.sh` beside it that
the script's own header requires — an extraction nothing exercises is a rule that now fires nowhere.
And honesty in `WORDS`: a comment is prose that changed file, not prose you saved, so words you moved
into a script are still there. A comment's bar is `~/.kk-flavor/standards/human-writing.md` → **Code comments**.

## Scope discipline

- Edit **only** your listed files. Other agents own the rest and run concurrently.
- A change another file needs is a `CROSS-SCOPE`, not a reach outside your scope.
- **Skip `kk-ecosystem`'s wiring check over the root** (its first and last steps) — that is your orchestrator's. Other agents are mid-edit and their in-flight state reads as findings; fixing it would clobber files you do not own.
- **Skip its shape and prose stages too** — it spawns `kk-skillcraft` and `kk-tighten`, and the campaign runs both later over what survived. Rule economy is the whole of your pass.
- Never delete a file unless your scope says to.
- Do not add rules. One you believe is missing is a `PROPOSAL`.

## On the target

Your word target is an **aim, not a quota**. Missing it and showing the arithmetic is a better outcome than hitting it by cutting instruction; the target itself may simply be wrong. Never reach a target by deleting something rescued.

## Return contract

Terse. No preamble, no narration of what you read.

```
SCOPE: <files>
WORDS: <before> -> <after> (saved N)
DELETED: one line per rule removed, each naming what still covers it — or plainly "nothing did"
MOVED: what you split, extracted, or gave one home, and where the pointer sits
RESCUED-KEPT: one line per plan-rescued passage, confirming it survives and where
CROSS-SCOPE: changes another file needs (path + what) — or "none"
HANDOFF: each lane your edits opened (skill + the files that opened it) — never kk-skillcraft or kk-tighten, the campaign runs those — or "none"
PROPOSAL: additions you did not make — or "none"
BROKEN: references your edits invalidated — or "none"
```

**A cut you applied but doubt belongs in `DELETED` with "nothing did".** That line is how the orchestrator finds what to restore; swallowing the doubt is how a real rule disappears quietly.
