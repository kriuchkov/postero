package tui

import "github.com/atotto/clipboard"

// clipboardWriteAll is a seam for tests; production uses the system clipboard.
var clipboardWriteAll = clipboard.WriteAll
