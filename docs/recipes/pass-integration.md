# Password Management in Postero

Storing passwords in plain text in `config.yaml` is not recommended for security reasons. Postero offers two better ways to manage credentials:

## 1. Built-in OS Keychain (Recommended)

Postero natively supports storing your passwords securely in your OS's keychain (macOS Keychain, Linux Secret Service / KWallet, Windows Credential Manager). 

You can manage these passwords directly via the Postero CLI:
```bash
# Save a password securely (it will prompt you to enter it safely)
pstr auth set "personal"

# Delete a saved password
pstr auth delete "personal"
```

If a password is set via `pstr auth`, you completely omit `password` and `password_cmd` from your `config.yaml`. Postero will automatically fetch it behind the scenes for the `personal` account.

---

## 2. Using External Password Managers (`password_cmd`)

If you prefer external CLI tools, Postero supports fetching passwords dynamically using shell commands via the `password_cmd` field.

### Using `pass` (The Standard Unix Password Manager)

If you use `pass` to store your email passwords, you can configure Postero to fetch the password by executing a shell command.

In your `config.yaml`, add the `password_cmd` field instead of `password`:

```yaml
accounts:
  - name: "personal"
    email: "user@example.com"
    username: "user@example.com"
    imap:
      host: "imap.gmail.com"
      port: 993
      tls: true
      password_cmd: ["pass", "show", "email/personal"]
    smtp:
      host: "smtp.gmail.com"
      port: 587
      tls: true
      password_cmd: ["pass", "show", "email/personal"]
```

### Using macOS `security` Tool

For macOS users preferring manual keychain commands:

```yaml
      password_cmd: ["security", "find-generic-password", "-w", "-a", "user@example.com", "-s", "imap.gmail.com"]
```

### Using other managers

You can use any command-line tool that returns the password to standard output:

- **Bitwarden** (via `bw`):
  `password_cmd: ["bw", "get", "password", "email_account_id"]`
- **1Password** (via `op`):
  `password_cmd: ["op", "read", "op://Personal/Email/password"]`
