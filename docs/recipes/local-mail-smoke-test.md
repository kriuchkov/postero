# Local IMAP/SMTP Smoke Test

This recipe starts a local GreenMail container and points Postero at it to verify the real SMTP send and IMAP sync paths.

## Start the test mail server

From the repository root:

```sh
docker compose -f docker-compose.mailtest.yml up -d
docker compose -f docker-compose.mailtest.yml ps
```

The stack exposes:

- SMTP on `127.0.0.1:3025`
- IMAP on `127.0.0.1:3143`
- GreenMail API/UI on `127.0.0.1:8080`

Configured test mailbox:

- login: `tester@test.local`
- password: `secret`

## Create a dedicated Postero config

Create a temporary config directory and file:

```yaml
accounts:
  - name: "local"
    email: "tester@test.local"
    username: "tester@test.local"
    password: "secret"
    imap:
      host: "127.0.0.1"
      port: 3143
      tls: false
    smtp:
      host: "127.0.0.1"
      port: 3025
      tls: false

storage:
  backend: "sqlite"

data_path: ".tmp/postero-mailtest"
```

Run Postero against that config by setting `POSTERO_CONFIG_DIR` to the directory containing `config.yaml`.

## Validate config

```sh
POSTERO_CONFIG_DIR="$PWD/.tmp/mailtest-config" ./bin/pstr config validate
```

## Send a real message over SMTP

```sh
POSTERO_CONFIG_DIR="$PWD/.tmp/mailtest-config" ./bin/pstr compose \
  --account local \
  --to tester@test.local \
  --subject "smtp smoke" \
  --body "hello from postero" \
  --send
```

Expected result: the command prints a sent message id and exits successfully.

## Fetch the same message over IMAP

```sh
POSTERO_CONFIG_DIR="$PWD/.tmp/mailtest-config" ./bin/pstr sync --account local
```

Expected result: Postero connects to IMAP, fetches the message from `INBOX`, and saves it into the local store.

## Inspect the local store

If the sqlite backend is enabled, the database is written under `.tmp/postero-mailtest/postero.db`.

## Stop the test mail server

```sh
docker compose -f docker-compose.mailtest.yml down -v
```

## What this actually verifies

- SMTP connection and envelope delivery
- SMTP payload generation
- IMAP login and mailbox select
- IMAP message fetch and parsing
- Config credential resolution for direct username/password auth

It does not verify OAuth2 flows, provider presets against public providers, or TLS certificate edge cases.