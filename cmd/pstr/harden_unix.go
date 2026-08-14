//go:build unix

package main

import "syscall"

// hardenFilePermissions restricts the process umask so newly created files are
// owner-only (0600) and directories 0700.
func hardenFilePermissions() {
	syscall.Umask(0o077)
}
