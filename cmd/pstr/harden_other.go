//go:build !unix

package main

// hardenFilePermissions is a no-op on platforms without a POSIX umask (e.g.
// Windows), where file access is governed by ACLs instead.
func hardenFilePermissions() {}
