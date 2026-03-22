# Connecting Outlook / Microsoft 365 to Postero

Microsoft supports both App Passwords (Legacy Auth) and OAuth2 for connecting IMAP/SMTP clients. Here's how to configure both.

For Outlook.com accounts, Postero understands `provider: "outlook"` and fills in the standard IMAP/SMTP settings automatically.

## Method 1: OAuth2 (Modern & Recommended)

Many corporate / school M365 environments require OAuth2 and have disabled legacy App Passwords. Postero natively brings modern XOAUTH2 support to solve this.

### 1. Configure Postero

In your `config.yaml`, set the account provider and provide your OAuth app settings:

```yaml
accounts:
  - name: "outlook"
    provider: "outlook"
    email: "your.name@outlook.com"
    oauth2:
      client_id: "your-client-id"
      client_secret: "your-client-secret"
```

That preset fills in these defaults:

- IMAP host: `outlook.office365.com`
- SMTP host: `smtp.office365.com`
- TLS: enabled
- Auth type: `oauth2`
- OAuth provider: `microsoft`
- Tenant ID: `common`
- OAuth scopes for IMAP, SMTP, and offline refresh

### 2. Run the Built-In Login Flow

```bash
pstr auth login outlook
```

If the account is not yet present in `config.yaml`, you can bootstrap it directly from the CLI:

```bash
pstr auth login outlook \
  --provider outlook \
  --email your.name@outlook.com \
  --client-id your-client-id \
  --client-secret your-client-secret
```

Or save the account first and start login in one step:

```bash
pstr auth add outlook \
  --email your.name@outlook.com \
  --client-id your-client-id \
  --client-secret your-client-secret \
  --login
```

Postero will ask you to open the authorization URL, paste back the code, and then store the token in the OS keychain.

When the OAuth2 preset is active for the account, Postero refreshes the saved token automatically and uses XOAUTH2 for IMAP and SMTP.

---

## Method 2: App Password (Legacy Auth)

If your account still supports Basic Authentication (usually personal Outlook.com / Hotmail accounts), you can use an App Password.

1. Go to your Microsoft Account settings (https://account.microsoft.com/security).
2. Go to **Advanced security options** and ensure Two-step verification is turned ON.
3. Scroll down to **App passwords** and click **Create a new app password**.
4. Configure your `config.yaml`:

```yaml
accounts:
  - name: "outlook"
    provider: "outlook"
    email: "your.name@outlook.com"
    password: "your-generated-app-password" # Consider using OS Keychain instead!
```

For better security, prefer `pstr auth set outlook` over storing the app password directly in `config.yaml`.