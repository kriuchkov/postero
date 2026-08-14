package main

import (
	"github.com/kriuchkov/postero/internal/adapters/commands/cli"
)

func main() {
	// Postero stores private data (emails, the local database and its WAL/-shm
	// sidecars, attachments, config). Force an owner-only umask so every file this
	// process creates is 0600 / dir 0700 and never readable by other local users,
	// regardless of how the C SQLite driver or third-party libraries create files.
	hardenFilePermissions()

	cli.Execute()
}
