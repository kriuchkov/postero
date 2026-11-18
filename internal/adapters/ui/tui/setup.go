package tui

import (
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/zalando/go-keyring"

	"github.com/kriuchkov/postero/internal/config"
)

// First-run account wizard: collect email and password, infer the provider
// preset from the domain, store the secret in the OS keychain, and sync.

const (
	setupFieldEmail = iota
	setupFieldPassword
	setupFieldIMAPHost
	setupFieldSMTPHost
)

// keyringSetPassword is a seam for tests; production writes the OS keychain.
var keyringSetPassword = keyring.Set

func appPasswordHint(provider string) string {
	switch provider {
	case "gmail":
		return "Gmail needs an app password: myaccount.google.com/apppasswords (2FA required)"
	case "outlook":
		return "Outlook may need an app password: account.live.com/proofs/apppassword"
	case "yahoo":
		return "Yahoo needs an app password: login.yahoo.com → Account Security"
	case "icloud":
		return "iCloud needs an app-specific password: appleid.apple.com → Sign-In & Security"
	case "fastmail":
		return "Fastmail: create an app password in Settings → Privacy & Security"
	case "yandex":
		return "Yandex needs an app password: id.yandex.com → Security → App passwords"
	default:
		return "Unknown domain — enter your IMAP/SMTP hosts below"
	}
}

func (m *Model) enterSetupState() {
	m.state = stateSetup
	m.setupFocus = setupFieldEmail
	m.setupErr = ""
	m.setupProvider = ""

	m.setupEmailInput = textinput.New()
	m.setupEmailInput.Placeholder = "you@example.com"
	m.setupEmailInput.Prompt = "> "

	m.setupPasswordInput = textinput.New()
	m.setupPasswordInput.Placeholder = "app password"
	m.setupPasswordInput.Prompt = "> "
	m.setupPasswordInput.EchoMode = textinput.EchoPassword
	m.setupPasswordInput.EchoCharacter = '•'

	m.setupIMAPInput = textinput.New()
	m.setupIMAPInput.Placeholder = "imap.example.com"
	m.setupIMAPInput.Prompt = "> "

	m.setupSMTPInput = textinput.New()
	m.setupSMTPInput.Placeholder = "smtp.example.com"
	m.setupSMTPInput.Prompt = "> "

	m.applySetupFocus()
}

// setupNeedsHosts reports whether manual IMAP/SMTP fields are required
// because the email domain has no provider preset.
func (m Model) setupNeedsHosts() bool {
	email := strings.TrimSpace(m.setupEmailInput.Value())
	return strings.Contains(email, "@") && config.ProviderForEmail(email) == ""
}

func (m *Model) applySetupFocus() {
	inputs := []*textinput.Model{&m.setupEmailInput, &m.setupPasswordInput, &m.setupIMAPInput, &m.setupSMTPInput}
	for index, input := range inputs {
		if index == m.setupFocus {
			input.Focus()
		} else {
			input.Blur()
		}
	}
}

func (m *Model) setupVisibleFields() []int {
	fields := []int{setupFieldEmail, setupFieldPassword}
	if m.setupNeedsHosts() {
		fields = append(fields, setupFieldIMAPHost, setupFieldSMTPHost)
	}
	return fields
}

func (m *Model) moveSetupFocus(step int) {
	fields := m.setupVisibleFields()
	position := 0
	for index, field := range fields {
		if field == m.setupFocus {
			position = index
			break
		}
	}
	position = (position + step + len(fields)) % len(fields)
	m.setupFocus = fields[position]
	m.applySetupFocus()
}

func (m Model) handleSetupKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case msg.Type == tea.KeyCtrlC:
		return m, tea.Quit
	case msg.Type == tea.KeyCtrlD:
		return m.enterDemoMode()
	case keyMatches(msg, m.keys.Esc):
		// The wizard is skippable; the app still works for browsing local mail.
		m.state = stateSidebar
		m.setStatus("Setup skipped — run :setup any time to add an account")
		return m, nil
	case msg.Type == tea.KeyEnter:
		return m.advanceSetup()
	case msg.Type == tea.KeyTab:
		m.moveSetupFocus(1)
		return m, nil
	case msg.Type == tea.KeyShiftTab:
		m.moveSetupFocus(-1)
		return m, nil
	}

	var cmd tea.Cmd
	switch m.setupFocus {
	case setupFieldEmail:
		m.setupEmailInput, cmd = m.setupEmailInput.Update(msg)
		m.setupProvider = config.ProviderForEmail(m.setupEmailInput.Value())
		m.setupErr = ""
	case setupFieldPassword:
		m.setupPasswordInput, cmd = m.setupPasswordInput.Update(msg)
		m.setupErr = ""
	case setupFieldIMAPHost:
		m.setupIMAPInput, cmd = m.setupIMAPInput.Update(msg)
		m.setupErr = ""
	case setupFieldSMTPHost:
		m.setupSMTPInput, cmd = m.setupSMTPInput.Update(msg)
		m.setupErr = ""
	}
	return m, cmd
}

// advanceSetup moves to the next field, or submits from the last one.
func (m Model) advanceSetup() (tea.Model, tea.Cmd) {
	email := strings.TrimSpace(m.setupEmailInput.Value())
	if m.setupFocus == setupFieldEmail {
		if !strings.Contains(email, "@") {
			m.setupErr = "Enter a valid email address"
			return m, nil
		}
		m.setupFocus = setupFieldPassword
		m.applySetupFocus()
		return m, nil
	}

	fields := m.setupVisibleFields()
	if m.setupFocus != fields[len(fields)-1] {
		m.moveSetupFocus(1)
		return m, nil
	}
	return m.submitSetup()
}

func (m Model) submitSetup() (tea.Model, tea.Cmd) {
	email := strings.TrimSpace(m.setupEmailInput.Value())
	password := m.setupPasswordInput.Value()
	if !strings.Contains(email, "@") {
		m.setupErr = "Enter a valid email address"
		m.setupFocus = setupFieldEmail
		m.applySetupFocus()
		return m, nil
	}
	if strings.TrimSpace(password) == "" {
		m.setupErr = "Enter the account password"
		m.setupFocus = setupFieldPassword
		m.applySetupFocus()
		return m, nil
	}

	account := config.AccountConfig{
		Name:     setupAccountName(email),
		Email:    email,
		Username: email,
	}
	if config.ProviderForEmail(email) == "" {
		imapHost := strings.TrimSpace(m.setupIMAPInput.Value())
		smtpHost := strings.TrimSpace(m.setupSMTPInput.Value())
		if imapHost == "" || smtpHost == "" {
			m.setupErr = "Enter IMAP and SMTP hosts for this provider"
			m.setupFocus = setupFieldIMAPHost
			if imapHost != "" {
				m.setupFocus = setupFieldSMTPHost
			}
			m.applySetupFocus()
			return m, nil
		}
		account.IMAP = config.IMAPConfig{Host: imapHost, Port: 993, TLS: true}
		account.SMTP = config.SMTPConfig{Host: smtpHost, Port: 587, TLS: true}
	}

	// Passwords are only ever stored in the OS keychain — never written to the
	// config file. If the keychain is unavailable (e.g. a headless session), we
	// refuse rather than fall back to plaintext, and point at the alternatives.
	if err := keyringSetPassword("postero", account.Name, password); err != nil {
		m.setupErr = "OS keychain unavailable — Postero will not store passwords in plaintext. " +
			"Configure a password_cmd for this account, then restart."
		return m, nil
	}

	cfg := config.UpsertAccount(m.config, account)
	if err := config.SaveConfig(cfg); err != nil {
		m.setupErr = "Could not save config: " + err.Error()
		return m, nil
	}

	m.config = cfg
	m.applyAccountsFromConfig(cfg)
	m.state = stateList
	cmd := m.refreshMailboxCmd()
	m.setStatus("Account " + email + " added (password in keychain) — syncing...")
	return m, cmd
}

// enterDemoMode loads a sample inbox so the app can be explored without a real
// account. It sets an in-memory demo account and starts seeding the store.
func (m Model) enterDemoMode() (tea.Model, tea.Cmd) {
	m.applyDemoAccount()
	m.state = stateList
	m.setStatus("Demo mode — loading sample inbox...")
	m.prepareFreshMessageFetch()
	return m, m.demoSeedCmd()
}

// setupAccountName derives a friendly account name from the email local part.
func setupAccountName(email string) string {
	local, _, ok := strings.Cut(email, "@")
	local = strings.TrimSpace(local)
	if !ok || local == "" {
		return "personal"
	}
	return local
}

func renderSetupView(m Model) string {
	labelStyle := lipgloss.NewStyle().Bold(true).Foreground(m.styles.Palette.Primary)
	textStyle := lipgloss.NewStyle().Foreground(m.styles.Palette.Text)
	subStyle := lipgloss.NewStyle().Foreground(m.styles.Palette.SubText)
	errStyle := lipgloss.NewStyle().Bold(true).Foreground(m.styles.Palette.Secondary)

	lines := []string{
		labelStyle.Render("Welcome to Postero"),
		subStyle.Render("Add your first email account — it takes under a minute."),
		"",
		textStyle.Render("Email"),
		m.setupEmailInput.View(),
	}

	email := strings.TrimSpace(m.setupEmailInput.Value())
	if strings.Contains(email, "@") {
		if m.setupProvider != "" {
			lines = append(lines, subStyle.Render("Detected provider: "+m.setupProvider))
		}
		lines = append(lines, subStyle.Render(appPasswordHint(m.setupProvider)))
	}

	lines = append(lines,
		"",
		textStyle.Render("Password"),
		m.setupPasswordInput.View(),
	)

	if m.setupNeedsHosts() {
		lines = append(lines,
			"",
			textStyle.Render("IMAP host (port 993, TLS)"),
			m.setupIMAPInput.View(),
			textStyle.Render("SMTP host (port 587, TLS)"),
			m.setupSMTPInput.View(),
		)
	}

	if m.setupErr != "" {
		lines = append(lines, "", errStyle.Render(m.setupErr))
	}
	lines = append(lines,
		"",
		labelStyle.Render("Just looking?")+subStyle.Render("  Press ctrl+d to try demo mode with a sample inbox."),
		subStyle.Render("enter next/submit • tab fields • ctrl+d demo • esc skip • ctrl+c quit"),
	)

	card := lipgloss.NewStyle().
		Padding(1, 3).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(m.styles.Palette.Primary).
		Render(lipgloss.JoinVertical(lipgloss.Left, lines...))

	if m.width > 0 && m.height > 0 {
		return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, card)
	}
	return card
}
