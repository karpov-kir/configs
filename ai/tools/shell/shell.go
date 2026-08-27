// Package shell is the shell semantics the ecosystem's Go tools are ported against: the primitives
// `check.sh` and `stats.sh` reached for through `tr`, `sed`, `basename`, `find` and `wc`, plus what
// their line-oriented scans read out of a markdown file.
//
// It exists because the shell side keeps its copies byte-identical under a drift scan and the Go side
// has none: that scan reads `*.sh` only, so a second Go copy of one of these could drift with nothing
// to notice. One implementation, every caller.
//
// Nothing here knows anything about the ecosystem, and that is the boundary. A checkout, the mount it
// is compared against, and the `@import` names that load alongside it are facts about one tree rather
// than about `sed`, so they live in `kk-flavor/tools/eco-root` with the root every path is built from.
// It is also why there is no readability test here: `[ -r ]` has two answers — open(2)'s and
// access(2)'s — and each caller keeps the one it means rather than inheriting a shared guess.
//
// The exact edges are the contract — which bytes count as a control byte, which space `s/^ //`
// removes, what `-type f` says about a symlink — so each of them is stated here once rather than
// re-derived at a call site. Nothing here holds state between calls, writes to a stream, or exits.
package shell
