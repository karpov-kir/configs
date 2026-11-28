---
name: kk-simplify
description: Simplify one whole repository hard, as a checkpointed multi-agent run in which every tracked file gets a verdict. Deletion comes first — what no reader needs goes. What stays is shortened, folded into one principle or source, or rebuilt in a simpler design, across code, architecture, tests, guards, config, docs, instructions and comments. Use for "simplify the repo", "this is overengineered", "clean up this repo", "we are bloating", or to resume an interrupted run. Lands as one PR, or as themed PRs. The whole tree in one bounded run: one change set is kk-qualify's or the built-in simplify's, the kk-flavor tree's own instructions are kk-reduce's, and a standing loop of single fixes is kk-patrol's.
argument-hint: "[one-pr | batches] (default: one-pr)"
disable-model-invocation: true
---

**Runs:** orchestrator
**Extends:** kk-qualify — Phase 5, over each PR branch's change set

Simplify the repository you stand in. `~/.kk-flavor/skills/kk-simplify/catalog.md` lists what the run looks for. The scouts find it, choose the verdict and design the simpler form. The cleaners build that form, with their own judgement and the project's checks. You keep the checkpoints, ask the human, and land the result.

You run under `~/.kk-flavor/standards/skill-protocol.md` as the orchestrator, so read it before Phase 1.

## Scope

The scope is every file in `origin/main`'s tree, plus the tails outside git (**Approval**, below). In the repository that ships the kk-flavor tree, leave that tree out of the inventory and out of every scout's assignment. It belongs to `kk-reduce`.

## Modes

The themes are code and architecture, tests and guards, config, docs, comments and instructions, and history. Each catalog class joins the theme that names it. Dependencies, CI, permissions and data join config. API surface, observability and names join code.

- `one-pr` is the default. The run's branch carries the single PR.
- `batches` opens one PR per theme. Split a theme whose diff a reviewer cannot read in one sitting.

## The checkpoint

Keep the run in `${XDG_STATE_HOME:-~/.local/state}/kk-flavor/simplify/<key>/`, where `~/.kk-flavor/scripts/repo-key.sh` prints `<key>`. The directory stays outside the repository, so another session can resume the run. Only you write in it.

- `run.md` holds the mode, the run's branch and worktree, the base SHA, the phase, and one row per slice: its paths, its status, its findings file and its attempts.
- `inventory.tsv` holds a header and one row per tracked file, in this column order: path, blob, slice, class, verdict, finding ids, and the commit that applied them. The class stays empty until the scout sets it. A verdict is `pending`, `disputed`, or a verdict from `~/.kk-flavor/workers/simplify/scout.md` → **The bar**.
- `findings/<slice>.md` holds each scout's return. You write it from the return.
- `decisions.md` holds the questions for the human and the answers.
- `batches.md` holds each theme's partitions, files, commits and PR, and each finding with its status: `open`, `applied`, `disputed` or `waiting` for the human.

**Resume** before you start anything. Without `run.md`, start at Phase 1. A dirty run worktree means a cleaner did not return: show the human its diff, and ask before you commit or discard it. With it, fetch, and continue at the recorded phase while `origin/main` still equals the base SHA. When `origin/main` has moved, merge it into the run's branch. A conflict in that merge stops the run and goes to the human. Compare `git ls-tree -r` of the old base and of `origin/main`, then record `origin/main`'s SHA as the new base. Each slice holding a changed, added or deleted file goes back to Phase 2, and so does every whole-tree assignment. Set all of that slice's rows to `pending` with the new base's blobs. The scouts read the run's branch, so they see what the run already applied. Each new findings file replaces the old one and takes fresh ids. An applied finding keeps its id and commit in `batches.md`.

## 1. Inventory

Start from a clean tree and a fresh `origin/main`. Create the run's branch and its worktree from `origin/main`, as `~/.kk-flavor/standards/git.md` → **Branches** names them. Write `inventory.tsv` from `git ls-tree -r origin/main`.

Cut the tree into slices. A slice is one module or directory that one scout can read whole. Split a directory that does not fit, and merge small siblings.

Tell the human the cost before the first spawn: the scout count, and the model the scout and cleaner rows resolve to. Ask in the same turn for the licence to commit on the run's branches, which Phases 4 and 5 use. Without it, each commit waits for the human's approval. The run pushes only in Phase 5.

## 2. Scout

Dispatch `~/.kk-flavor/workers/simplify/scout.md` once per slice, and three more times over the whole tree: for `rules`, `structure` and `tails`. The scouts read only, so run them all at once. Write each return into its findings file and each verdict into `inventory.tsv` as it arrives. A file two scouts judge keeps the earlier verdict in the order of `~/.kk-flavor/workers/simplify/scout.md` → **The bar**. Phase 2 is done when every assignment has returned and every row in `inventory.tsv` holds a verdict past `pending`. Resume a scout whose return leaves a file of its slice without a verdict line, and count the attempt in `run.md`.

## 3. Sort

Group the findings by theme, then into partitions inside each theme. Each file belongs to one partition, and every finding sits whole in one. A file that two themes need goes, with all its findings, to the theme of its first finding. Size each partition so that one cleaner can hold all of it in its context. A `structure` redesign is one partition of its own in the code and architecture theme.

A finding that needs the human, each tail, and each product feature the scouts propose to drop go to `decisions.md`. Ask the whole list in one turn, with your recommendation and the exact command for each item. Run each command the human approves. The rest go on at once.

## 4. Clean

Dispatch `~/.kk-flavor/workers/simplify/cleaner.md` once per partition, theme by theme, in the run's worktree, with its findings. Run one cleaner at a time, so that no two cleaners edit the worktree at once.

After each cleaner returns, commit its edits on the run's branch, one commit per partition with its theme in the subject. Record the commit in `batches.md` and in `inventory.tsv`. Set a disputed file's verdict to `disputed`. A cleaner's proposal and a cleaner's new finding go back to Phase 3 as findings. A new finding that needs the human is asked when it arrives.

Run the repository's own gate after the last cleaner. A red gate, and a red check a cleaner found before its first edit, stop Phase 4 and go to the human.

## 5. Land

Land only once `inventory.tsv` holds no `pending` row and every finding is applied, `disputed`, or waiting in `decisions.md`.

In `one-pr`, the PR branch is the run's branch. In `batches`, create one branch and worktree per theme from `origin/main`, and cherry-pick that theme's commits onto it. A conflict in a cherry-pick stops the run and goes to the human.

In each PR branch's worktree, run the repository's gate. A red gate on a theme branch stops the run and goes to the human, with the commits from another theme it needs. Then run `~/.kk-flavor/skills/kk-qualify/SKILL.md` over the branch's change set, with its applied findings as the ask, and commit its fixes on that branch. Push, and open each PR as a draft, as `~/.kk-flavor/standards/git.md` states.

Each PR description says what was simplified and why. It gives the counts before and after: lines, files, dependencies, and the CI run time of `origin/main` against the branch. It gives each `structure` redesign its own section with the reason. It lists each `disputed` finding, each scout's follow-up and catalog proposal, and each open question from `decisions.md`.

The human merges.

## Approval

Ask the human before you touch anything git cannot restore, and wait for the answer:

- an untracked file, a stash, a worktree, or an unmerged or unpushed branch;
- user data, a fixture holding real data, and a migration that drops a table, column or key holding rows;
- a behaviour a user of the product sees.

Everything else in git goes without a question. History restores it, and the human reads the PR before the merge. That includes specific rules folded into principles and weak principles deleted.

## Close

Once every PR is open and every question is answered or listed in a PR, remove the run's worktrees and the local branches the PRs carry. Then delete the run's directory. The PRs are then the record. The closing reply names each PR and each question still open.
