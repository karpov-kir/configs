package aibootstrap

import "syscall"

// Whether this process may read and write the file, which is access(2)'s answer and `[ -r ] && [ -w ]`'s.
// Asked of the kernel rather than read off the mode bits, because root ignores them and a capability
// grants them without one.
func isReadableAndWritable(path string) bool {
	const readable, writable = 0x4, 0x2
	return syscall.Access(path, readable|writable) == nil
}
