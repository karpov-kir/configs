#!/usr/bin/env bash
#
# The kk-flavor region both installers write into a CLAUDE.md they do not own: two fences and the
# lines between them. lib/owned-region.sh is the mechanism and knows nothing about what a region
# says, so the one body this repository asks it to write is held here rather than in each installer.
#
# The owner tier does not come through here: it mounts ai/CLAUDE.md at ~/.claude/CLAUDE.md, and a
# mounted file stands alone, so that file's copy of these lines cannot be derived from this one.
#
# tested by: ai/bootstrap-test.sh and ai/install-project-test.sh, because a region is only real once
# an install has written one.
set -uo pipefail

flavor_region_open="<!-- kk-flavor:begin -->"
flavor_region_close="<!-- kk-flavor:end -->"

flavor_region_body() {
  cat <<'BODY'
### KK Flavor

Read `~/.kk-flavor/inject.md` now and follow it — applies to all work, skill-invoked or ad-hoc.
BODY
}
