package installer

// The link-count reader regionWritable consults, so a case can produce the one answer no machine here
// can: a count that could not be read. The shell reached that state routinely — `stat -c` is GNU's
// format flag and `-f` is BSD's, and on Linux `stat -f` prints a block of filesystem facts a numeric
// comparison reads as one link, so a write went through a hardlink to somebody's private file. Go asks
// the kernel, and the only way left is a filesystem whose stat is not the platform's.
func StubLinkCount(run *Run, count func(path string) (uint64, error)) {
	run.tree.linkCount = count
}
