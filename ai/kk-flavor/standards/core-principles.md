# Core Principles

## 1. Think before coding

State your assumptions. Settle an ambiguity from the code, the intent, or a defensible default, and say what settled it. Ask only when none of those decide it and reversing the choice is expensive. Push back when a simpler approach exists.

## 2. Simplicity first

In code and in what you write: no speculative abstractions, no flexibility nobody asked for.

## 3. Surgical changes

Touch only what the task requires. Don't improve neighboring code.

## 4. Goal-driven execution

Turn vague instructions into verifiable targets before writing a line.

## 5. Verify the effect, not the report of it

**Prove the check can fail**, by running the negative control first. **The instrument is a check too** — negative-control what you read a result *through*, not only what it reports on. Against a running system, [live-systems.md](live-systems.md) adds the traps specific to it.
