package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"

	appcore "github.com/kriuchkov/postero/internal/app"
	"github.com/kriuchkov/postero/internal/config"
	"github.com/kriuchkov/postero/internal/core/models"
	"github.com/kriuchkov/postero/internal/core/ports"
)

// SessionState defines the active pane.
type SessionState int

const (
	stateSidebar SessionState = iota
	stateList
	stateContent
	stateCompose
)

//nolint:recvcheck // Bubble Tea models intentionally mix value and pointer receivers.
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
	assistant        ports.DraftAssistant
	allMessages      []*models.Message
	sidebarTagSource []*models.Message
	messages         []*models.Message
	listCursor       int
	fetchOffset      int
	hasMoreMessages  bool
	messagesLoading  bool
	aiGenerating     bool
	loadingFrame     int
	loadingToken     int
	aiLoadingFrame   int
	aiLoadingToken   int
	aiLoadingLabel   string
	activeDraft      *models.Message // For compose/reply
	accountNames     []string
	accountEmails    map[string]string
	defaultFrom      string
	defaultAcctID    string
	activeAccountID  string
	activeTagID      string
	statusMessage    string
	statusError      bool
	composeTitle     string
	composeHint      string
	composeEditing   bool
	composeBaseline  *models.Message
	searchInput      textinput.Model
	commandActive    bool
	commandDraft     string
	commandHistory   []string
	commandHistoryIx int
	searchActive     bool
	searchQuery      string
	searchDebouncing bool
	searchToken      int
	pendingUndo      *undoState
	undoToken        int
	contentViewport  viewport.Model
	contentMessageID string
	pendingMotion    string
	pendingCount     string
	lastAction       repeatableAction

	// Compose inputs
	toInput      textinput.Model
	subjectInput textinput.Model
	bodyInput    textarea.Model
	focusIndex   int // 0: Account, 1: To, 2: Subject, 3: Body
}

// initialModel wires the starting UI state together with config-derived services, theme, and compose inputs.
func initialModel() Model {
	items := []string{"Inbox", "Sent", "Drafts", "Archive", "Trash", "Spam"}

	cfg, err := appcore.LoadConfig()
	var msgService ports.MessageService
	var draftAssistant ports.DraftAssistant
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

		if service, _, serviceErr := appcore.NewMessageService(); serviceErr == nil {
			msgService = service
		}
		if assistant, assistantErr := appcore.NewDraftAssistantWithConfig(cfg); assistantErr == nil {
			draftAssistant = assistant
		}
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
		config:           cfg,
		state:            stateSidebar,
		keys:             bindings,
		help:             help.New(),
		styles:           styles,
		sidebarItems:     items,
		sidebarCursor:    0,
		service:          msgService,
		assistant:        draftAssistant,
		allMessages:      []*models.Message{},
		sidebarTagSource: []*models.Message{},
		messages:         []*models.Message{},
		listCursor:       0,
		fetchOffset:      0,
		hasMoreMessages:  false,
		messagesLoading:  false,
		aiGenerating:     false,
		loadingFrame:     0,
		loadingToken:     0,
		aiLoadingFrame:   0,
		aiLoadingToken:   0,
		aiLoadingLabel:   "",
		activeDraft:      nil,
		accountNames:     accountNames,
		accountEmails:    accountEmails,
		defaultFrom:      defaultFrom,
		defaultAcctID:    defaultAcctID,
		activeAccountID:  "",
		activeTagID:      "",
		statusMessage:    "",
		statusError:      false,
		composeTitle:     "",
		composeHint:      "",
		composeEditing:   false,
		commandActive:    false,
		commandDraft:     "",
		commandHistory:   nil,
		commandHistoryIx: -1,
		searchInput: func() textinput.Model {
			input := textinput.New()
			input.Prompt = "/ "
			input.Placeholder = "subject, sender, body"
			return input
		}(),
		searchActive:     false,
		searchQuery:      "",
		searchDebouncing: false,
		searchToken:      0,
		contentViewport:  viewport.New(0, 0),
		contentMessageID: "",
		pendingMotion:    "",
		pendingCount:     "",
		lastAction:       repeatableActionNone,
		toInput:          textinput.New(),
		subjectInput:     textinput.New(),
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
