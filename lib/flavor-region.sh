#!/usr/bin/env bash
#
# The kk-flavor region both installers write into an agent instruction file they do not own: two fences and the
# lines between them. lib/owned-region.sh is the mechanism and knows nothing about what a region
# says, so the one body this repository asks it to write is held here rather than in each installer.
#
# The owner tier copies ai/owner-instructions.md to the selected client's user instruction file.
# That standalone template carries these lines itself.
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
