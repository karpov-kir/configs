// Package shell is the shell semantics the ecosystem's Go tools are ported against: the primitives
// `check.sh` and `stats.sh` reached for through `tr`, `sed`, `basename`, `find` and `wc`, and the
// `# --- shared:<region> ---` blocks those two scripts keep byte-identical on purpose.
//
// It exists because that byte-identity is a property the shell side has a drift scan for and the Go
// side does not: check.sh's shared-region scan reads `*.sh` only, so a second Go copy of `imports_in`
// or of `contained_in_root` could drift with nothing to notice. One implementation, two callers.
//
// The exact edges are the contract — which bytes count as a control byte, which space `s/^ //`
// removes, what `-type f` says about a symlink — so each of them is stated here once rather than
// re-derived at a call site. Nothing here holds state between calls, writes to a stream, or exits.
package shell
