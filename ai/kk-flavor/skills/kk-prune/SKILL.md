---
name: kk-prune
description: Prune one repository hard — every tracked file gets a verdict, and what nobody needs is deleted or cut down — docs, comments, code, overbuilt design, config, tests, guards, specific rules, wrong facts and history. Use for "prune the repo", "clean up this repo", "we are bloating". Lands as one PR or as themed PRs. One change set's quality is kk-qualify's, and the kk-flavor tree's own instructions are kk-reduce's.
argument-hint: "[one-pr | batches] (default: one-pr)"
disable-model-invocation: true
---

**Runs:** orchestrator
**Extends:** kk-qualify — Phase 5, over each branch's change set

Prune the repository you stand in. The scouts find and justify. The cleaners edit. You keep the checkpoints, ask the human, and land the result.

You run under `~/.kk-flavor/standards/skill-protocol.md` as the orchestrator, so read it before Phase 1.

## Scope

The scope is every file `git ls-files` lists, plus the tails outside git (**Approval**, below). In the repository that ships the kk-flavor tree, leave that tree out of the inventory. It belongs to `kk-reduce`.

## Modes

- `one-pr` is the default. The run lands on one branch, with one commit per theme.
- `batches` lands one PR per theme, each on its own branch and worktree from `origin/main`.

The themes are code and config, tests and guards, docs, comments and instructions, and history. Split a theme whose diff a reviewer cannot read in one sitting.

## The checkpoint

Keep the run in `${XDG_STATE_HOME:-~/.local/state}/kk-flavor/prune/<key>/`, where `~/.kk-flavor/scripts/repo-key.sh` prints `<key>`. The directory stays outside the repository, so another session can resume the run. Only you write in it.

- `run.md` holds the mode, the run's branches, the base SHA, the phase, and one row per slice: its paths, its status, its findings file and its attempts.
- `inventory.tsv` holds a header and one row per tracked file, in this column order: path, blob, slice, class, verdict, finding ids, and the commit that applied them. The class stays empty until the scout sets it. A verdict is `pending`, `keep`, `delete`, `rewrite`, `simplify` or `disputed`.
- `findings/<slice>.md` is the file each scout writes.
- `decisions.md` holds the questions for the human and the answers.
- `batches.md` holds each theme's partitions, files, branch, commits and PR, and each finding with its status: `open`, `applied`, `disputed` or `waiting` for the human.

**Resume** before you start anything. Without `run.md`, start at Phase 1. With it, fetch, and continue at the recorded phase while `origin/main` still equals the base SHA. When `origin/main` has moved, merge it into each of the run's branches. A conflict in that merge stops the run and goes to the human. Compare `git ls-tree -r` of the old base and of `origin/main`, then record `origin/main`'s SHA as the new base. Each slice holding a changed, added or deleted file goes back to Phase 2, and so does every whole-tree assignment. Set all of that slice's rows to `pending` with the new base's blobs. The scouts read the run's branch, so they see what the run already applied. Each new findings file replaces the old one and takes fresh ids. An applied finding keeps its id and commit in `batches.md`.

## 1. Inventory

Start from a clean tree and a fresh `origin/main`. Create the run's branch from `origin/main`, or in `batches` one branch and worktree per theme, as `~/.kk-flavor/standards/git.md` → **Branches** names them. Then write `inventory.tsv` from `git ls-tree -r origin/main`.

Cut the tree into slices. A slice is one module or directory that one scout can read whole. Split a directory that does not fit, and merge small siblings.

Tell the human the cost before the first spawn: the scout count, and the model the scout and cleaner rows resolve to.

## 2. Scout

Dispatch `~/.kk-flavor/workers/prune/scout.md` once per slice, and three more times over the whole tree: for `rules`, for `structure` and for `tails`. The scouts read only, so run them all at once. Write each verdict into `inventory.tsv` as it arrives.

## 3. Sort

Group the findings by theme, then into partitions inside each theme. Each file belongs to one partition, and every finding sits whole in one. A file that two themes need goes, with all its findings, to the theme of its first finding. A finding that needs the human, and each tail, goes to `decisions.md` and waits. The rest go on at once.

## 4. Clean

Dispatch `~/.kk-flavor/workers/prune/cleaner.md` once per partition, with its findings file paths. Partitions share no file, so run them at once. In `batches`, give each theme's partitions that theme's worktree.

Record each returned finding's status in `batches.md`, and set a disputed file's verdict to `disputed` in `inventory.tsv`. A `disputed` finding keeps its file.

Run the repository's own gate after the cleaners of a branch return. A red gate, a cleaner's proposal and a cleaner's new finding go back to Phase 3 as findings.

## 5. Land

Land only once `inventory.tsv` holds no `pending` row and every finding is applied, `disputed`, or waiting in `decisions.md`. Then run `~/.kk-flavor/skills/kk-qualify/SKILL.md` over each branch's change set, with that branch's applied findings as the ask. Then commit, push and open a draft PR as `~/.kk-flavor/standards/git.md` states. In `one-pr`, make one commit per theme.

Each PR description says what went and why, and the line count before and after. It lists each `disputed` finding, each scout's follow-up, and each open question from `decisions.md`.

The human merges.

## Approval

Ask the human before you touch anything git cannot restore, and wait for the answer:

- an untracked file, a stash, a worktree, or an unmerged or unpushed branch;
- user data, or a fixture holding real data;
- a behaviour a user of the product sees.

Everything else in git goes without a question. History restores it, and the human reads the PR before the merge. That includes specific rules folded into principles.

Ask the whole list in one turn after Phase 2, with your recommendation and the exact command for each item. Run each command the human approves.

## Close

Delete the run's directory once every PR is open and every question is answered or listed in a PR. The PRs are then the record. The closing reply names each PR and each question still open.
