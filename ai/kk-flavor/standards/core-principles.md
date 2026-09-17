**Layer:** base

## 1. Think first

State your assumptions. Settle an ambiguity from the code, from the intent, or from a defensible default, and say what settled it. Ask the human only where none of those three decide it and reversing the choice is expensive. Push back where a simpler approach exists.

## 2. Simplicity first

Write no speculative abstraction and no flexibility the task did not ask for, in code and in what you write. A safeguard obeys this too. Weigh what a safeguard costs the deliverable — a dependency, a toolchain, a CI job — against what it prevents, and drop the ones that cost more. Legacy code obeys this too. Replace the old shape and move everything that depends on it, leaving no compatibility shim, alias or flag beside the new one. Bring the choice to keep both shapes to the human. Do not take that choice yourself.

## 3. Surgical changes

Touch only what the task requires.

## 4. Goal-driven execution

Turn vague instructions into verifiable targets before writing a line.

## 5. Verify the effect, not the report of it

Prove the check can fail by running the negative control first. Run the negative control through the same instrument as the subject, because a run elsewhere is no evidence about the run you are reading. Take the negative control from a check you already needed. Do not invent a subject that can go red in order to have one. Treat the instrument and the subject as checks too: a result read through something that never ran looks exactly like a result, and so does a sound reading of the wrong thing. A pipe hands you its last command's status, so `check | head` reports `head`'s and an unrun check reads as clean. Do not pipe a command whose status you need, or set `-o pipefail` first.
