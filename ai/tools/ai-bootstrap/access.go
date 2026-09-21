package aibootstrap

import "syscall"

// Whether this process may read and write the file, which is what access(2) and `[ -r ] && [ -w ]` answer.
// Root ignores the mode bits and a capability grants access without them, so the kernel is asked.
func isReadableAndWritable(path string) bool {
	const readable, writable = 0x4, 0x2
	return syscall.Access(path, readable|writable) == nil
}
