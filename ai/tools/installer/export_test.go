package installer

// The link-count reader RegionWritable consults, so a case can produce the one answer no machine here
// can: a count that could not be read.
func StubLinkCount(run *Run, count func(path string) (uint64, error)) {
	run.tree.linkCount = count
}
