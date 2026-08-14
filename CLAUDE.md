# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project

Postero (`pstr`) is a terminal-first email client (TUI + CLI) written in Go 1.25. Module path: `github.com/kriuchkov/postero`. Single binary built from `cmd/pstr`.

## Commands

```bash
make build              # build ./bin/pstr
make run                # build + run the TUI
make test               # unit tests in Docker (go test -short, coverage → coverage.out/html)
make integration-test   # go test ./tests/integration/... (testcontainers; needs Docker)
make linter             # golangci-lint v2.12.2 in Docker with --fix
make man                # regenerate man pages into ./docs/man
make mail-smoke-test    # end-to-end SMTP/IMAP smoke test against GreenMail (Docker)
```

Local (non-Docker) equivalents:

```bash
go build -o bin/pstr ./cmd/pstr
go test -short ./...                         # unit tests only (integration tests are gated by -short)
go test -run TestName ./internal/services/message/   # single test
go test ./tests/integration/...              # integration tests (spins up mail containers)
```

Mocks are generated with mockery (`.mockery.yaml`) into `internal/core/ports/mocks`. Regenerate with `mockery` after changing a port interface.

## Architecture

Hexagonal / clean architecture. Dependencies point inward toward `core`; adapters and services depend on `core/ports` interfaces, never the reverse.

- **`internal/core`** — the domain, no external I/O.
  - `models/` — domain types (`Message`, AI request/response types) and service request/response structs.
  - `ports/` — all interfaces (`MessageRepository`, `MessageService`, `IMAPRepository`, `SMTPRepository`, `DraftAssistant`, `PromptCompletionProvider`). This is the contract every adapter and service implements.
  - `errors/` — domain errors via `DomainError{Code, Message, Err}` and constructors like `MessageNotFound`, `AccountNotFound`, `PasswordNotConfigured`.
- **`internal/services`** — business logic implementing the `*Service` ports.
  - `message/` — core email operations; `NewServiceWithSMTP` injects an SMTP factory so sends resolve per-account.
  - `assistant/` — AI draft generation over `PromptCompletionProvider` implementations.
- **`internal/adapters`** — implementations of ports, grouped by responsibility:
  - `commands/cli/` — Cobra commands (one file per subcommand; registered in `root.go`). Running `pstr` with no subcommand launches the TUI.
  - `mail/imap/`, `mail/smtp/` — mail transport.
  - `storage/sqlite/`, `storage/file/` — two `MessageRepository` backends, selected at runtime by `storage.backend` (`sqlite` default, `file` = JSON-on-disk).
  - `ai/openai/`, `ai/gemini/` — `PromptCompletionProvider` implementations.
  - `ui/tui/` — Bubble Tea TUI (Elm-style `Model`/`Update`/`View`; keymap, panes, styles).
- **`internal/app`** — composition root. `runtime.go` wires config → repository → services → assistant and holds account/provider resolution (`ResolveAccount`, `smtpFactory`, `NewDraftAssistantWithConfig`). Add new backends/providers to the `switch` statements here.
- **`internal/config`** — Viper-based config loading, provider presets, OAuth2, keychain access, validation.
- **`pkg/`** — reusable, import-safe helpers (`compose/`, `format/`).

### Adding functionality

- New email operation: add the method to the `MessageService` (and `MessageRepository` if it needs persistence) port in `core/ports`, implement in `services/message`, expose via a `cli/` command and/or the TUI.
- New storage backend or AI provider: implement the port, then register it in the `switch` in `internal/app/runtime.go`.

## Configuration

Config is loaded via Viper with env prefix `POSTERO` (e.g. `POSTERO_PASSWORD`, `POSTERO_<ACCOUNT>_SMTP_PASSWORD`). `POSTERO_CONFIG_DIR` overrides the config directory (used by the smoke test). Secrets can come from the config file, `password_cmd`, env vars, or the OS keychain (`zalando/go-keyring`). See `postero.yaml.example`.

## Conventions

- Wrap errors with `github.com/go-faster/errors` (`errors.Wrap`/`errors.Errorf`); return typed domain errors from `core/errors` for known failure modes.
- Linting is strict (`.golangci.yaml`): `depguard` bans `math/rand` (use `math/rand/v2`), `github.com/golang/protobuf`, and unmaintained uuid packages in non-test code. Run `make linter` before finishing.

## Note

`.dev/aerc/` is a vendored read-only copy of the aerc email client (separate module `git.sr.ht/~rjarry/aerc`) kept for reference only. It is not part of the build — do not modify it or count it as project code.
