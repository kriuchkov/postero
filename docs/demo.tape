# Postero README demo — drives the TUI through first-run demo mode.
# Render with: vhs docs/demo.tape   (or: make demo)
# Requires: vhs (github.com/charmbracelet/vhs), and ./bin/pstr already built.

Output docs/demo.gif

Set Shell "bash"
# Menlo ships with macOS (no install) and covers Cyrillic + box-drawing.
# Swap to any installed monospace, e.g. "Cascadia Code", "IBM Plex Mono",
# "Fira Code", or VHS's bundled default "JetBrains Mono".
Set FontFamily "Menlo"
Set FontSize 11
Set Width 1060
Set Height 560
Set Padding 14
Set Theme "Catppuccin Mocha"
Set TypingSpeed 60ms
Set PlaybackSpeed 1.0

# Launch against a fresh config dir so the first-run setup wizard appears. Seed a
# tiny AI config (a local command agent, standing in for OpenClaw / Claude Code)
# so the reply popup (A) works in demo mode.
Hide
Type `export POSTERO_CONFIG_DIR=$(mktemp -d) && cp demo/config.yaml "$POSTERO_CONFIG_DIR/config.yaml" && ./bin/pstr`
Enter
Sleep 2s
Show

# The welcome / setup wizard: new users land here.
Sleep 2.5s

# One key into demo mode — a sample inbox, no account needed.
Ctrl+D
Sleep 3s

# Open the highlighted welcome message — Markdown **bold** renders as bold.
Enter
Sleep 3s

# Reply with an AI agent: press A for a popup, type an instruction, Enter.
Type "A"
Sleep 1.8s
Type "accept warmly and mention I'll set up my account today"
Sleep 2s
Enter
Sleep 2.8s

# The agent's reply lands in the composer above the readable quote, cursor at top.
Sleep 1.2s

# Discard this draft (it's just a demo).
Escape
Sleep 700ms
Type ":q!"
Enter
Sleep 1.4s

# Walk a couple more messages.
Down
Sleep 900ms
Down
Sleep 900ms

# vim-style folder navigation: goto Sent, then back to Inbox.
Type "gs"
Sleep 1.8s
Type "gi"
Sleep 1.6s

# Start a compose, then cancel it.
Type "c"
Sleep 2s
Escape
Sleep 800ms

# Quit with a vim-style command.
Type ":q"
Enter
Sleep 1.2s
