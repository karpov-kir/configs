// Package flavor is the region both installers write into an agent instruction file they do not own:
// two fences and the lines between them.
//
// Apart from the mounting machinery on purpose. installer's region writer knows nothing about what a
// region says, which is what lets one implementation serve a markdown instruction file and a
// .gitignore alike; the one body this repository asks it to write lives here instead of in each
// installer, so the two cannot drift.
//
// The owner tier copies ai/owner-instructions.md rather than writing a region, and that standalone
// template carries these lines itself. ai/tools/ai-bootstrap's suite holds the two against each other.
package flavor

// The fences. HTML comments, so the region is invisible when the markdown renders.
const (
	RegionOpen  = "<!-- kk-flavor:begin -->"
	RegionClose = "<!-- kk-flavor:end -->"
)

// RegionBody is what goes between them.
const RegionBody = "### KK Flavor\n\n" +
	"Read `~/.kk-flavor/inject.md` now and follow it — applies to all work, skill-invoked or ad-hoc."

// The fences an older Codex install wrote around its RTK note. Nothing writes this region any more;
// an uninstall still removes it, because the machines that have one are the machines being uninstalled
// and nothing else would ever take it out.
const (
	LegacyRtkRegionOpen  = "<!-- kk-flavor-rtk:begin -->"
	LegacyRtkRegionClose = "<!-- kk-flavor-rtk:end -->"
)
