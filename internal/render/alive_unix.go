//go:build unix

package render

import (
	"errors"
	"syscall"
)

// alive reports whether a process with the given pid exists. Signal 0 runs the
// existence and permission checks without delivering anything; EPERM means the
// process exists but belongs to another user.
func alive(pid int) bool {
	err := syscall.Kill(pid, 0)
	return err == nil || errors.Is(err, syscall.EPERM)
}
