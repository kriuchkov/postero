package tui

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	appcore "github.com/kriuchkov/postero/internal/app"
	"github.com/kriuchkov/postero/internal/config"
	"github.com/kriuchkov/postero/internal/core/models"
	"github.com/kriuchkov/postero/internal/core/ports"
)

// SessionState defines the active pane
type SessionState int

const (
	stateSidebar SessionState = iota
	stateList
	stateContent
	stateCompose
)

type keyMap struct {
	Up        key.Binding
	Down      key.Binding
	Left      key.Binding
	Right     key.Binding
	Refresh   key.Binding
	Search    key.Binding
	MarkRead  key.Binding
	Undo      key.Binding
	PageUp    key.Binding
	PageDown  key.Binding
	HalfUp    key.Binding
	HalfDown  key.Binding
	Top       key.Binding
	Bottom    key.Binding
	Quit      key.Binding
	Compose   key.Binding
	Reply     key.Binding
	ReplyAll  key.Binding
	Forward   key.Binding
	Archive   key.Binding
	Spam      key.Binding
	Delete    key.Binding
	Edit      key.Binding
	Send      key.Binding
	SaveDraft key.Binding
	Download  key.Binding
	Esc       key.Binding
	Enter     key.Binding
}

func (k keyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Up, k.Down, k.Left, k.Right, k.Search, k.MarkRead, k.Quit}
}

func (k keyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Up, k.Down, k.Left, k.Right},
		{k.Search, k.Refresh, k.MarkRead},
		{k.Undo},
		{k.PageUp, k.PageDown, k.HalfUp, k.HalfDown},
		{k.Top, k.Bottom},
		{k.Compose, k.Reply, k.ReplyAll, k.Forward},
		{k.Archive, k.Spam, k.Delete, k.SaveDraft},
		{k.Edit, k.Send},
		{k.Quit},
	}
}

func defaultKeyMap() keyMap {
	return keyMap{
		Up:        key.NewBinding(key.WithKeys("up", "k"), key.WithHelp("k/↑", "up")),
		Down:      key.NewBinding(key.WithKeys("down", "j"), key.WithHelp("j/↓", "down")),
		Left:      key.NewBinding(key.WithKeys("left", "h"), key.WithHelp("h/←", "left")),
		Right:     key.NewBinding(key.WithKeys("right", "l"), key.WithHelp("l/→", "right")),
		Refresh:   key.NewBinding(key.WithKeys("ctrl+r"), key.WithHelp("ctrl+r", "refresh")),
		Search:    key.NewBinding(key.WithKeys("/"), key.WithHelp("/", "search")),
		MarkRead:  key.NewBinding(key.WithKeys("m"), key.WithHelp("m", "mark read")),
		Undo:      key.NewBinding(key.WithKeys("u"), key.WithHelp("u", "undo")),
		PageUp:    key.NewBinding(key.WithKeys("pgup", "ctrl+b"), key.WithHelp("pgup", "page up")),
		PageDown:  key.NewBinding(key.WithKeys("pgdown", "ctrl+f"), key.WithHelp("pgdn", "page down")),
		HalfUp:    key.NewBinding(key.WithKeys("ctrl+u"), key.WithHelp("ctrl+u", "half up")),
		HalfDown:  key.NewBinding(key.WithKeys("ctrl+d"), key.WithHelp("ctrl+d", "half down")),
		Top:       key.NewBinding(key.WithKeys("home"), key.WithHelp("gg/home", "top")),
		Bottom:    key.NewBinding(key.WithKeys("end", "G"), key.WithHelp("G/end", "bottom")),
		Quit:      key.NewBinding(key.WithKeys("q", "ctrl+c"), key.WithHelp("q", "quit")),
		Compose:   key.NewBinding(key.WithKeys("c"), key.WithHelp("c", "compose")),
		Reply:     key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "reply")),
		ReplyAll:  key.NewBinding(key.WithKeys("R"), key.WithHelp("R", "reply all")),
		Forward:   key.NewBinding(key.WithKeys("f"), key.WithHelp("f", "forward")),
		Archive:   key.NewBinding(key.WithKeys("a"), key.WithHelp("a", "archive")),
		Spam:      key.NewBinding(key.WithKeys("!"), key.WithHelp("!", "spam")),
		Delete:    key.NewBinding(key.WithKeys("delete", "d", "backspace"), key.WithHelp("d", "delete")),
		Edit:      key.NewBinding(key.WithKeys("i"), key.WithHelp("i", "edit field")),
		Send:      key.NewBinding(key.WithKeys("ctrl+x"), key.WithHelp("ctrl+x", "send")),
		SaveDraft: key.NewBinding(key.WithKeys("ctrl+o"), key.WithHelp("ctrl+o", "save draft")),
		Download:  key.NewBinding(key.WithKeys("s"), key.WithHelp("s", "save attachments")),
		Esc:       key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "cancel")),
		Enter:     key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "select/new line")),
	}
}

func keyMapFromConfig(cfg config.KeybindingsConfig) keyMap {
	bindings := defaultKeyMap()
	bindings.Quit = newBinding(cfg.Quit, []string{"q", "ctrl+c"}, "quit")
	bindings.Refresh = newBinding(cfg.Refresh, []string{"ctrl+r"}, "refresh")
	bindings.Compose = newBinding(cfg.Compose, []string{"c"}, "compose")
	bindings.Reply = newBinding(cfg.Reply, []string{"r"}, "reply")
	bindings.Forward = newBinding(cfg.Forward, []string{"f"}, "forward")
	bindings.Search = newBinding(cfg.Search, []string{"/"}, "search")
	bindings.Delete = newBinding(cfg.Delete, []string{"delete", "d", "backspace"}, "delete")
	bindings.MarkRead = newBinding(cfg.MarkRead, []string{"m"}, "mark read")
	return bindings
}

func newBinding(raw string, fallback []string, description string) key.Binding {
	keys := parseBindingKeys(raw, fallback)
	return key.NewBinding(key.WithKeys(keys...), key.WithHelp(keys[0], description))
}

func parseBindingKeys(raw string, fallback []string) []string {
	if strings.TrimSpace(raw) == "" {
		return fallback
	}
	parts := strings.Split(raw, ",")
	keys := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			keys = append(keys, trimmed)
		}
	}
	if len(keys) == 0 {
		return fallback
	}
	return keys
}

type Model struct {
	config           *config.Config
	state            SessionState
	keys             keyMap
	help             help.Model
	styles           Styles
	width            int
	height           int
	sidebarItems     []string
	sidebarCursor    int
	service          ports.MessageService
	allMessages      []*models.MessageDTO
	messages         []*models.MessageDTO
	listCursor       int
	activeDraft      *models.MessageDTO // For compose/reply
	accountNames     []string
	accountEmails    map[string]string
	defaultFrom      string
	defaultAcctID    string
	activeAccountID  string
	statusMessage    string
	statusError      bool
	composeTitle     string
	composeHint      string
	composeEditing   bool
	searchInput      textinput.Model
	searchActive     bool
	searchQuery      string
	pendingUndo      *undoState
	undoToken        int
	contentViewport  viewport.Model
	contentMessageID string
	pendingMotion    string

	// Compose inputs
	toInput      textinput.Model
	subjectInput textinput.Model
	bodyInput    textarea.Model
	focusIndex   int // 0: Account, 1: To, 2: Subject, 3: Body
}

func initialModel() Model {
	items := []string{"Inbox", "Sent", "Drafts", "Archive", "Trash", "Spam"}

	cfg, err := appcore.LoadConfig()
	var msgService ports.MessageService
	defaultAcctID := ""
	defaultFrom := ""
	accountNames := []string{}
	accountEmails := map[string]string{}

	if err == nil && cfg != nil {
		defaultAcctID, defaultFrom = appcore.DefaultSender(cfg)
		for _, acc := range cfg.Accounts {
			accountNames = append(accountNames, acc.Name)
			accountEmails[acc.Name] = acc.Email
		}
		if len(cfg.Accounts) > 0 {
			items = append(items, "")
			items = append(items, "Accounts:")
			for _, acc := range cfg.Accounts {
				items = append(items, fmt.Sprintf("  %s", acc.Name))
			}
		}

		msgService, _, err = appcore.NewMessageService()
	}

	if msgService == nil {
		msgService, _, _ = appcore.NewMessageService()
	}

	bindings := defaultKeyMap()
	styles := DefaultStyles()
	if cfg != nil {
		bindings = keyMapFromConfig(cfg.Keybindings)
		styles = StylesFromTheme(cfg.Theme)
	}

	return Model{
		config:          cfg,
		state:           stateSidebar, // Start at sidebar
		keys:            bindings,
		help:            help.New(),
		styles:          styles,
		sidebarItems:    items,
		sidebarCursor:   0,
		service:         msgService,
		allMessages:     []*models.MessageDTO{},
		messages:        []*models.MessageDTO{},
		listCursor:      0,
		activeDraft:     nil,
		accountNames:    accountNames,
		accountEmails:   accountEmails,
		defaultFrom:     defaultFrom,
		defaultAcctID:   defaultAcctID,
		activeAccountID: "",
		statusMessage:   "",
		statusError:     false,
		composeTitle:    "",
		composeHint:     "",
		composeEditing:  false,
		searchInput: func() textinput.Model {
			input := textinput.New()
			input.Prompt = "Search: "
			input.Placeholder = "subject, sender, body"
			return input
		}(),
		searchActive:     false,
		searchQuery:      "",
		contentViewport:  viewport.New(0, 0),
		contentMessageID: "",
		pendingMotion:    "",

		toInput:      textinput.New(),
		subjectInput: textinput.New(),
		bodyInput: func() textarea.Model {
			input := textarea.New()
			input.ShowLineNumbers = false
			return input
		}(),
		focusIndex: 0,
	}
}

func (m Model) Init() tea.Cmd {
	return m.fetchMessages()
}

func keyMatches(msg tea.KeyMsg, k key.Binding) bool {
	return key.Matches(msg, k)
}

type messagesLoadedMsg struct {
	messages        []*models.MessageDTO
	targetCursor    int
	targetID        string
	activeAccountID string
}

type undoState struct {
	message   *models.MessageDTO
	action    string
	token     int
	expiresAt time.Time
}

type undoExpiredMsg struct {
	token int
}

func (m Model) fetchMessages() tea.Cmd {
	return m.fetchMessagesAtCursor(-1)
}

func (m Model) fetchMessagesAtCursor(targetCursor int) tea.Cmd {
	return m.fetchMessagesSelection(targetCursor, "")
}

func (m Model) fetchMessagesForID(targetID string) tea.Cmd {
	return m.fetchMessagesSelection(-1, targetID)
}

func (m Model) fetchMessagesSelection(targetCursor int, targetID string) tea.Cmd {
	return func() tea.Msg {
		if m.service == nil {
			return nil
		}

		ctx := context.Background()
		var msgs []*models.MessageDTO
		var err error

		// Determine what to fetch based on sidebar selection
		if m.sidebarCursor >= len(m.sidebarItems) {
			return nil
		}
		selectedItem := m.sidebarItems[m.sidebarCursor]
		scopeAccountID := strings.TrimSpace(m.activeAccountID)
		if accountID, ok := m.selectedAccountID(); ok {
			scopeAccountID = accountID
		}

		// Basic mapping
		switch selectedItem {
		case "Inbox":
			msgs, err = m.fetchScopedMailbox(ctx, scopeAccountID, "Inbox")
		case "Sent":
			msgs, err = m.fetchScopedMailbox(ctx, scopeAccountID, "Sent")
		case "Drafts":
			msgs, err = m.fetchScopedMailbox(ctx, scopeAccountID, "Drafts")
		case "Archive":
			msgs, err = m.fetchScopedMailbox(ctx, scopeAccountID, "Archive")
		case "Trash":
			msgs, err = m.fetchScopedMailbox(ctx, scopeAccountID, "Trash")
		case "Spam":
			msgs, err = m.fetchScopedMailbox(ctx, scopeAccountID, "Spam")
		default:
			if selectedItem == "" || selectedItem == "Accounts:" {
				return nil
			}
			if accountID, ok := m.selectedAccountID(); ok {
				msgs, err = m.fetchScopedMailbox(ctx, accountID, "Inbox")
			} else {
				msgs, err = m.service.GetByLabel(ctx, strings.TrimSpace(selectedItem), 100, 0)
			}
		}

		if err != nil {
			return nil
		}

		return messagesLoadedMsg{messages: msgs, targetCursor: targetCursor, targetID: targetID, activeAccountID: scopeAccountID}
	}
}

func (m Model) fetchScopedMailbox(ctx context.Context, accountID, mailbox string) ([]*models.MessageDTO, error) {
	accountID = strings.TrimSpace(accountID)
	if accountID == "" {
		switch mailbox {
		case "Inbox":
			return m.service.GetAllInboxes(ctx, 100, 0)
		case "Sent":
			return m.service.GetSent(ctx, 100, 0)
		case "Drafts":
			return m.service.GetDrafts(ctx, 100, 0)
		case "Archive":
			return m.service.GetByLabel(ctx, "archive", 100, 0)
		case "Trash":
			isDeleted := true
			return m.service.SearchMessages(ctx, models.SearchCriteria{IsDeleted: &isDeleted, Limit: 100})
		case "Spam":
			isSpam := true
			isDeleted := false
			return m.service.SearchMessages(ctx, models.SearchCriteria{IsSpam: &isSpam, IsDeleted: &isDeleted, Limit: 100})
		default:
			return nil, nil
		}
	}

	isDeleted := false
	criteria := models.SearchCriteria{AccountID: accountID, IsDeleted: &isDeleted, Limit: 100}
	switch mailbox {
	case "Inbox":
		isDraft := false
		isSpam := false
		criteria.IsDraft = &isDraft
		criteria.IsSpam = &isSpam
		criteria.Labels = []string{"inbox"}
	case "Sent":
		criteria.Labels = []string{"sent"}
	case "Drafts":
		isDraft := true
		criteria.IsDraft = &isDraft
	case "Archive":
		criteria.Labels = []string{"archive"}
	case "Trash":
		isDeleted = true
		criteria.IsDeleted = &isDeleted
	case "Spam":
		isSpam := true
		criteria.IsSpam = &isSpam
	default:
		return nil, nil
	}

	return m.service.SearchMessages(ctx, criteria)
}

func (m Model) selectedAccountID() (string, bool) {
	if m.sidebarCursor < 0 || m.sidebarCursor >= len(m.sidebarItems) {
		return "", false
	}
	selectedItem := m.sidebarItems[m.sidebarCursor]
	if !strings.HasPrefix(selectedItem, "  ") {
		return "", false
	}
	accountID := strings.TrimSpace(selectedItem)
	for _, accountName := range m.accountNames {
		if strings.EqualFold(accountName, accountID) {
			return accountName, true
		}
	}
	return "", false
}
