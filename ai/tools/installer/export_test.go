package installer

// StubLinkCount replaces the linkCount reader RegionWritable consults. A case can then produce an
// result no machine here gives. That result is a count that could not be read.
func StubLinkCount(run *Run, count func(path string) (uint64, error)) {
	run.tree.linkCount = count
}
