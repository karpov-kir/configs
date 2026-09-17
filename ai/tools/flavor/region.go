// Package flavor is the region both installers write into an agent instruction file they do not own:
// two fences and the lines between them. The owner tier copies ai/owner-instructions.md rather than
// writing a region, and that standalone template carries these lines itself. ai/tools/ai-bootstrap's
// suite holds the two against each other.
package flavor

const (
	RegionOpen  = "<!-- kk-flavor:begin -->"
	RegionClose = "<!-- kk-flavor:end -->"
)

const RegionBody = "### KK Flavor\n\n" +
	"Read `~/.kk-flavor/inject.md` now and follow it — applies to all work, skill-invoked or ad-hoc."

// The fences an older Codex install wrote around its RTK note. Nothing writes this region any more;
// an uninstall still removes it, because the machines that have one are the machines being uninstalled
// and nothing else would ever take it out.
const (
	LegacyRtkRegionOpen  = "<!-- kk-flavor-rtk:begin -->"
	LegacyRtkRegionClose = "<!-- kk-flavor-rtk:end -->"
)
