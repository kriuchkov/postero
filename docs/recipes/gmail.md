# Connecting Gmail to Postero

Postero supports both OAuth2 and App Passwords for Gmail. OAuth2 is the preferred setup when you have your own Google OAuth client configuration; App Passwords remain a simpler fallback.

For Gmail, Postero understands `provider: "gmail"` and fills in the standard IMAP/SMTP settings automatically.

## Method 1: OAuth2

Configure your account with the Gmail provider preset and your OAuth client settings:

```yaml
accounts:
  - name: "gmail"
    provider: "gmail"
    email: "your.name@gmail.com"
    oauth2:
      client_id: "your-client-id"
      client_secret: "your-client-secret"
```

`client_id` and `client_secret` come from your own Google Cloud OAuth application. Postero does not generate them for you.

Typical setup:

1. Open the Google Cloud Console.
2. Create or select a project.
3. Enable the Gmail API for that project.
4. Open **APIs & Services** -> **Credentials**.
5. Create an OAuth client ID, usually a Desktop app for local CLI usage.
6. Copy the generated client ID and client secret into your Postero config or pass them via `pstr auth login` / `pstr auth add` flags.

That preset fills in these defaults:

- IMAP host: `imap.gmail.com`
- SMTP host: `smtp.gmail.com`
- TLS: enabled
- Auth type: `oauth2`
- OAuth provider: `google`
- OAuth scope: `https://mail.google.com/`

Then run:

```bash
pstr auth login gmail
```

If the account is not yet present in `config.yaml`, you can bootstrap it directly from the CLI:

```bash
pstr auth login gmail \
  --provider gmail \
  --email your.name@gmail.com \
  --client-id your-client-id \
  --client-secret your-client-secret
```

Or save the account first and start login in one step:

```bash
pstr auth add gmail \
  --email your.name@gmail.com \
  --client-id your-client-id \
  --client-secret your-client-secret \
  --login
```

Postero stores the OAuth2 token in the OS keychain and refreshes it automatically when needed.

## Method 2: App Password

1. Go to your Google Account settings at https://myaccount.google.com/.
2. Open **Security**.
3. Make sure **2-Step Verification** is enabled.
4. Open **App passwords**.
5. Create a new app password for Postero.

You can either store it in the keychain:

```bash
pstr auth set gmail
```

or configure it directly:

```yaml
accounts:
  - name: "gmail"
    provider: "gmail"
    email: "your.name@gmail.com"
    password: "your-16-character-app-password"
```

If you prefer an external password manager, use `password_cmd` as described in [pass-integration.md](pass-integration.md).