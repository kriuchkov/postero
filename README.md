# Postero

[Features](#features) • [Quick Start](#quick-start) • [Navigation](#navigation) •
[Commands](#commands) • [Configuration](#configuration) • [Architecture](#architecture) • [License](#license)

## Features

**Postero** (pstr) is a modern open-source terminal email client designed for productivity. Built from the ground up for developers, engineers, and command-line aficionados, Postero combines the power of TUI with the simplicity and clarity you expect from modern workflows.

Features include:

- **Intuitive text-based interface** - Reading, replying, forwarding, and organizing email from the terminal
- **Modern authentication** - IMAP/SMTP accounts with app passwords, `password_cmd`, native OS keychain storage, and built-in OAuth2 login
- **Fast search and MIME filters** - Keyboard-centric navigation with configurable HTML-to-text rendering
- **Interactive TUI** - Bubble Tea based workflow for inboxes, drafts, attachments, focused reading, and vim-style navigation
- **Keyboard-first workflow** - Built-in shortcuts for triage, search, compose, and pane navigation
- **Account-aware mail flow** - Per-account sync, compose, reply, and send flows from CLI and TUI
- **Composable drafts and attachments** - Save drafts locally, inspect attachments, and download them from the TUI

## Quick Start

### Installation

#### Go Install

```bash
go install github.com/kriuchkov/postero/cmd/pstr@latest
```

#### Build from Source

```bash
go build -o pstr ./cmd/pstr
```

#### Download Binary

Download the latest release from the [Releases](https://github.com/kriuchkov/postero/releases) page.

### Basic Usage

Start Postero:

```bash
pstr
```

Running `pstr` without subcommands opens the interactive TUI.

Sync mailbox:

```bash
pstr sync
```

Search emails:

```bash
pstr search "subject:golang"
```

Compose new email:

```bash
pstr compose
```

Generate a compose or reply draft with AI using configured templates:

```bash
pstr compose ai --account Gmail --to user@example.com --instruction "Draft a short project kickoff email"
pstr reply ai msg-001 --template reply-default --instruction "Politely accept and ask for the agenda" --all
```

## Navigation

The TUI is keyboard-first and now supports vim-style movement across the sidebar, message list, reader pane, and composer.

### Global Movement

- `h` / `l` or `←` / `→` - Move focus between sidebar, message list, and reader pane
- `j` / `k` or `↓` / `↑` - Move within the active pane
- Prefix motions with counts such as `5j`, `3gg`, or `2G` to move multiple rows or jump to a specific visible item
- `gg` or `Home` - Jump to the top of the active pane
- `G` or `End` - Jump to the bottom of the active pane
- `0` - Jump to the start of the active pane
- `$` - Jump to the end of the active pane
- `Ctrl+u` / `Ctrl+d` - Half-page up or down
- `PgUp` / `PgDn` or `Ctrl+b` / `Ctrl+f` - Full-page movement

### Search And Mailbox Flow

- `/` - Start live search in the current mailbox
- `Enter` - Keep the current filtered result or open the selected draft
- `Esc` - Clear the active search or clear account scoping from the sidebar

### Message Actions

- `c` - Compose a new message
- `r` - Reply
- `R` - Reply all
- `f` - Forward
- `d` - Move to trash, or permanently delete in Trash
- `a` - Archive
- `!` - Mark as spam
- `.` - Repeat the last archive, trash, spam, or permanent delete action on the current selection
- `u` - Undo the last delete, archive, or spam action while the undo window is active
- `s` - Save attachments from the selected message to `~/Downloads`

### Command Mode

- `:` - Open the command palette
- Supported commands: `compose`, `compose-ai`, `reply-ai`, `reply-all-ai`, `inbox`, `sent`, `drafts`, `archive`, `trash`, `spam`, `refresh`, `help`, `quit`
- AI commands open the normal composer with generated content, for example `:compose-ai Draft a short kickoff email`, `:compose-ai --template compose-default Draft a short kickoff email`, or `:reply-ai --template reply-default Politely accept and confirm`
- While AI generation is in flight, the header and footer show a dedicated loading badge so network-backed draft generation is visible
- While a compose draft has unsaved changes, commands that would abandon it are blocked until you save, send, or cancel it

### Compose Mode

Compose has a normal mode for navigation and a writing mode for text entry.

- `j` / `k` - Move between Account, To, Subject, and Body while in normal mode
- `h` / `l` or `←` / `→` on the Account field - Switch the sending account
- Counts also work in compose normal mode, for example `2j`, `3gg`, or `G`
- `gg`, `G`, `0`, `$` - Jump to the first or last compose field
- `i` or `Enter` - Enter writing mode for the selected field
- `:` - Open the command palette from compose, including AI drafting commands such as `compose-ai --template compose-default ...`
- `Esc` - Leave writing mode; press `Esc` again to cancel compose
- `Ctrl+o` - Save draft
- `Ctrl+x` - Send message

## Configuration

Postero supports a flexible configuration system using YAML files and environment variables.

**Priority Order:**

1. Command-line flags
2. Environment variables
3. Configuration file (`~/.config/postero/config.yaml` or `./config.yaml`)
4. Default values

### Configuration File

Example `~/.config/postero/config.yaml`:

```yaml
accounts:
  - name: "personal"
    provider: "gmail"
    email: "user@example.com"
    username: "user@example.com"
    # For common providers, Postero fills IMAP/SMTP defaults automatically.
    imap:
      # password: "imap-app-password"
      # password_cmd: ["pass", "show", "email/personal-imap"]
    smtp:
      # password: "smtp-app-password"
      # password_cmd: ["pass", "show", "email/personal-smtp"]
    # Optional shared fallback if IMAP/SMTP passwords are the same
    # password: "shared-app-password"
    # password_cmd: ["pass", "show", "email/personal"]
    oauth2:
      client_id: "your-client-id"
      client_secret: "your-client-secret"

filters:
  # Render HTML emails using w3m
  text/html: "w3m -T text/html -dump"
  # Optional plain text post-processing
  # text/plain: "sed -e 's/\\r$//'"

tui:
  # Messages fetched per page in the interactive list and search results.
  list_page_size: 30
  # How close the cursor gets to the bottom before the next page is fetched.
  list_prefetch_ahead: 5
  # Spinner frame interval for loading indicators, in milliseconds.
  loading_tick_ms: 120
```

If `username` is omitted, Postero uses `email` as the login.

For common public providers, `provider: "gmail"` and `provider: "outlook"` prefill the standard IMAP/SMTP hosts, ports, TLS, and OAuth2 defaults.

For real IMAP and SMTP access, Postero resolves credentials in this order:

1. Refreshed OAuth2 tokens for accounts configured with `auth_type: oauth2`
2. `password_cmd` at protocol or account level
3. Native OS keychain entries saved with `pstr auth set` or `pstr auth login`
4. Environment variables
5. Inline config passwords

Environment variable fallbacks:

- `POSTERO_<ACCOUNT_NAME>_IMAP_PASSWORD`, for example `POSTERO_OUTLOOK_IMAP_PASSWORD`
- `POSTERO_<ACCOUNT_NAME>_SMTP_PASSWORD`, for example `POSTERO_OUTLOOK_SMTP_PASSWORD`
- `POSTERO_IMAP_PASSWORD` and `POSTERO_SMTP_PASSWORD` as protocol-wide fallbacks
- `POSTERO_<ACCOUNT_NAME>_PASSWORD` or `POSTERO_PASSWORD` as shared fallbacks for both protocols

The same env override convention applies to TUI settings. Examples:

- `POSTERO_TUI_LIST_PAGE_SIZE=50`
- `POSTERO_TUI_LIST_PREFETCH_AHEAD=8`
- `POSTERO_TUI_LOADING_TICK_MS=90`

These override the corresponding YAML keys under `tui:`.

If `imap.username` or `smtp.username` is omitted, Postero falls back to `username`, then to `email`.

`sync` and `compose --send` now use the configured IMAP/SMTP servers directly and return a clear error if credentials are missing.

Useful auth and config commands:

```bash
pstr config init gmail
pstr config validate
pstr auth add personal --provider gmail --email user@example.com
pstr auth set personal
pstr auth login personal
pstr auth delete personal
```

`pstr auth add` saves or updates an account entry in `config.yaml`. `pstr auth login` performs the OAuth2 code exchange inside Postero, stores the resulting token in the OS keychain, and can also bootstrap missing OAuth client settings from CLI flags.

## Commands

### Main Commands

- `pstr` - Launch the interactive terminal UI
- `sync` - Synchronize emails with IMAP server
- `search` - Search emails by subject, sender, or content
- `show` - Print one message with headers, labels, attachments, and body
- `compose` - Create and send new email
- `reply` - Reply to selected email
- `forward` - Forward email
- `list` - Print a mailbox snapshot to stdout
- `read` - Mark a message as read
- `star` - Toggle the starred state of a message
- `archive` - Move a message out of Inbox into Archive
- `trash` - Mark a message as trashed without deleting it permanently
- `delete` - Permanently remove a message from the local store
- `spam` - Mark a message as spam

`auth` subcommands manage saved credentials and OAuth2 logins:

- `auth set <account>` - Save a password in the OS keychain
- `auth add <provider>` - Create or update a provider-backed account in `config.yaml`
- `auth login <account>` - Run the built-in OAuth2 login flow and save the token in keychain
- `auth delete <account>` - Remove stored credentials for the account

`config` subcommands help initialize and validate YAML configuration:

- `config init <provider>` - Print a starter config snippet for a known provider
- `config validate` - Check the loaded config and print actionable validation hints

`compose`, `reply`, `forward`, and `sync` support `--account` so you can explicitly choose the configured account by name or email.

`compose ai` and `reply ai` use `ai.providers` and `ai.templates` from the config file. Templates are rendered with compose/reply context and must return JSON with `subject` and `body`. Use `--instruction` for the high-level request and `--var key=value` for extra template data.

`list` supports mailbox and output filters such as `--mailbox`, `--label`, `--limit`, and `--format`. `search` supports `--account`, `--label`, `--limit`, `--unread`, and `--format` for scripting-friendly usage.

Message action commands such as `read`, `star`, `archive`, `trash`, `spam`, and `delete` accept multiple IDs and can read IDs from stdin with `--stdin-ids` for shell pipelines. `trash` is reversible mailbox state, while `delete` permanently removes messages from the local store.

Examples:

```bash
pstr sync --account Outlook
pstr list --mailbox archive --limit 10
pstr search invoice --account Gmail --unread
pstr show msg-001
pstr compose --account Gmail --to user@example.com --subject "Hello" --attach ./invoice.pdf
pstr reply msg-001 --account Gmail --all --send
pstr trash msg-001 msg-002
pstr search invoice --format json | jq -r '.[].id' | pstr archive --stdin-ids
pstr delete msg-999
pstr archive msg-001
```

## Architecture

Postero follows Clean Architecture principles with a clear separation of concerns:

- **Entities/Models** - Core email message types
- **Use Cases/Services** - Business logic for email operations
- **Interface Adapters** - Grouped by responsibility: commands, mail, storage, and UI
- **Frameworks** - Cobra for CLI, Bubble Tea for TUI

### Directory Structure

```text
postero/
├── cmd/
│   └── pstr/
│       └── main.go
├── internal/
│   ├── adapters/
│   │   ├── commands/
│   │   │   └── cli/      # Cobra commands and CLI entrypoints
│   │   ├── mail/
│   │   │   ├── imap/     # IMAP transport adapter
│   │   │   └── smtp/     # SMTP transport adapter
│   │   ├── storage/
│   │   │   ├── file/     # JSON file-backed storage adapter
│   │   │   └── sqlite/   # SQLite-backed storage adapter
│   │   └── ui/
│   │       └── tui/      # Bubble Tea terminal UI
│   ├── app/              # Runtime wiring and factories
│   ├── config/           # Configuration management
│   ├── core/
│   │   ├── models/       # Domain models plus service request/response types
│   │   ├── errors/       # Domain errors
│   │   └── ports/        # Interfaces
│   └── services/
│       └── message/      # Email operations service
└── go.mod
```

Runtime wiring supports both SQLite and file-backed storage through `storage.backend`. Use `sqlite` for the default database-backed mode or `file` for JSON-on-disk storage.

## License

GPL-3.0-or-later
