# Core Principles

1. Think before coding, and state your assumptions. Settle an ambiguity from the code, the intent, or a defensible default and say what settled it; ask only when none of those decide it and reversing the choice is expensive. A question carries your recommended answer, the legwork behind it, and a number where the stakes are a size or a duration. Push back when a simpler approach exists.
2. Simplicity first, in code and in what you write: the minimum that solves the problem — no speculative abstractions, no flexibility nobody asked for.
3. Surgical changes. Touch only what the task requires. Don't improve neighboring code.
4. Goal-driven execution. Turn vague instructions into verifiable targets before writing a line.
5. Verify the effect, not the report of it — and **prove the check can fail**, by running the negative control first. Against a running system, [live-systems.md](live-systems.md) adds the traps specific to it.
