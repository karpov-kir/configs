// Package flavor is the region both installers write into an agent instruction file they do not own:
// two fences and the lines between them. The owner tier writes no region. It copies
// ai/owner-instructions.md, and that standalone template carries these lines itself. The suite in
// ai/tools/ai-bootstrap holds the two copies against each other.
package flavor

const (
	RegionOpen  = "<!-- kk-flavor:begin -->"
	RegionClose = "<!-- kk-flavor:end -->"
)

const RegionBody = "### KK Flavor\n\n" +
	"Read `~/.kk-flavor/inject.md` now and follow it — applies to all work, skill-invoked or ad-hoc."

// The fences an older Codex install wrote around its RTK note. No installer writes this region today.
// Uninstall still removes it, because a machine that carries one is a machine being uninstalled, and
// no other code path would take it out.
const (
	LegacyRtkRegionOpen  = "<!-- kk-flavor-rtk:begin -->"
	LegacyRtkRegionClose = "<!-- kk-flavor-rtk:end -->"
)
