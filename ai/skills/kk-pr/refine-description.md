# `refine-description` — rewrite the body, and the title where it no longer fits

1. **Read the change, not the body you are replacing.** The diff, the commits, and the ticket or intent the branch links. A description rewritten from the old description keeps whatever was wrong with it.
2. **Rewrite the body** to the standard `~/.claude/skills/kk-pr/SKILL.md` → **Land it** step 1 names for this mode.
3. **Leave the title alone unless it no longer names the change**, and leave a body alone that still fits. A rewrite for taste is a change a reviewer has to re-read for nothing.
4. **`gh pr edit <N> --body-file <file>`**, plus `--title` where step 3 changed it. Pass the body as a file: inline it is a shell argument, and a backtick in it runs as a command.

**A body someone has already reviewed is replaced openly** — say in your closing reply what you took out, because a reviewer who read the old one has no diff to read.
