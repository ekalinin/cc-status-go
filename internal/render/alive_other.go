//go:build !unix

package render

// alive has no syscall-cheap check outside unix, so a session record is taken
// at its word there.
func alive(int) bool { return true }
