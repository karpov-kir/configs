# Core Principles

## 1. Think before coding

State your assumptions. Settle an ambiguity from the code, the intent, or a defensible default, and say what settled it. Ask only when none of those decide it and reversing the choice is expensive. Push back when a simpler approach exists.

## 2. Simplicity first

In code and in what you write: no speculative abstractions, no flexibility nobody asked for. **A safeguard is not exempt.** Weigh what a safeguard costs the deliverable — a dependency, a toolchain, a CI job — against what it prevents, and drop the ones that cost more.

## 3. Surgical changes

Touch only what the task requires. Don't improve neighboring code.

## 4. Goal-driven execution

Turn vague instructions into verifiable targets before writing a line.

## 5. Verify the effect, not the report of it

**Prove the check can fail**, by running the negative control first. **A negative control is a property of a check you already needed** — never a reason to invent a subject that can go red. **The instrument and the subject are checks too** — a result read through something that never ran, and a sound reading of the wrong thing, both look exactly like results. Against a running system, [live-systems.md](live-systems.md) adds the traps specific to it.
