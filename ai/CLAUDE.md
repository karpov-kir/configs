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

- Never add NOSONAR or similar inline lint/Sonar suppression comments. Kirill resolves unfixable Sonar findings manually in the SonarCloud UI — if a finding can't be fixed in code, leave it and report it instead.
