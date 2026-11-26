package installer

// StubLinkCount replaces linkCount, the field of tree FileWritable consults. A case can then
// produce a result no machine here gives. That result is a count that could not be read.
func StubLinkCount(run *Run, count func(path string) (uint64, error)) {
	run.tree.linkCount = count
}

// Refusals is every refusal the run collected, which a case counts and prints.
func (r *Run) Refusals() []string {
	return r.refusals
}
