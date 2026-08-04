# Standards (kk-flavor)

Read `~/.kk-flavor/inject.md` now and follow it — applies to all work, skill-invoked or ad-hoc. The first time you load it in a session, open that message with this banner so I can see it's active:

```
🍦 kk-flavor loaded 🍦
```

# Caveman Mode

If caveman mode is active (startup hook sets it), display this banner as the first thing in the first message of the session:

```
🦴 caveman mode active 🦴
```

# Memory

Add new memory entries to this section. Do **not** create or write to per-project memory dirs (`~/.claude/projects/*/memory/`) — keep all memory here.

This is a staging area. Once enough memory accumulates, the user folds entries into the proper sections of this document. Do not reorganize on your own. Entries here are authoritative — apply them as if they were in a structured section.

- **A quality stage's own non-trivial fix is unreviewed code.** When a review stage applies a behaviour-changing fix of any size, nothing in the pipeline then reviews it for correctness — the stage that wrote it is not a second opinion. So a stage applying such a fix must land a test per branch it introduces. Observed: a code-review stage authored ~52 lines of an abort seam, and a defect in it (an already-aborted path removing an event listener it had never added) survived until a later refactor pass happened to look.

- **A change that starts depending on an in-repo sibling puts that sibling's public surface in review scope.** Scoping review to "what git says changed" misses the file the change newly leans on, even when that file plainly violates a documented rule. Observed: `runPeriodically` began delegating to `callWithTimeout`, whose 3-positional-param signature already broke the named-params rule; no stage raised it and the user did, twice mid-flight, growing the change set from 2 files to 5.

- **Considered and not built: a `positional-params.sh` check for `idsd-qualify`.** A grep over a diff's added exported signatures for 3+ positional params, riding refactor's evidence slot like `dup-literals.sh` does. Declined as marginal on one observed miss — revisit only if positional-param signatures recur across runs.
