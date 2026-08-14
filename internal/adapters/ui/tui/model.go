package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"

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
	stateSetup
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
	store            ports.MessageRepository
	assistant        ports.DraftAssistant
	syncer           ports.MailboxSyncer
	seeder           ports.MailboxSeeder
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
	// composeDiscardArmed is set after the first esc on an unsaved draft; the
	// next esc discards it.
	composeDiscardArmed bool
	searchInput         textinput.Model
	// AI reply popup: a modal prompt to hand the agent an instruction.
	aiPromptActive     bool
	aiPromptReplyAll   bool
	aiPromptInput      textinput.Model
	commandActive      bool
	commandDraft       string
	commandHistory     []string
	commandHistoryIx   int
	searchActive       bool
	searchQuery        string
	searchDebouncing   bool
	searchToken        int
	pendingUndo        *undoState
	undoToken          int
	contentViewport    viewport.Model
	contentMessageID   string
	renderedBody       string // cached getFilteredBody result for renderedBodyID
	renderedBodyID     string // message ID the cached body belongs to
	wrappedBody        string // renderedBody soft-wrapped to wrappedBodyWidth
	wrappedBodyWidth   int    // viewport width the wrapped body was built for
	wrappedBodyExpand  bool   // whether the cached wrap used expanded (full) URLs
	expandURLs         bool   // reader shows full URLs instead of collapsed labels
	pendingMotion      string
	pendingCount       string
	lastAction         repeatableAction
	announceSearchKind searchByKind
	announceSearchTerm string
	visualActive       bool
	visualAnchor       int

	// Compose inputs
	toInput      textinput.Model
	subjectInput textinput.Model
	bodyInput    textarea.Model
	focusIndex   int // 0: Account, 1: To, 2: Subject, 3: Body

	// First-run account setup wizard
	setupEmailInput    textinput.Model
	setupPasswordInput textinput.Model
	setupIMAPInput     textinput.Model
	setupSMTPInput     textinput.Model
	setupFocus         int
	setupProvider      string
	setupErr           string
}

// Dependencies carries everything the TUI needs, assembled by the composition
// root. The UI adapter depends only on these ports (and config), never on the
// wiring package, so the dependency arrow points inward.
type Dependencies struct {
	Config    *config.Config
	Service   ports.MessageService
	Store     ports.MessageRepository
	Assistant ports.DraftAssistant
	Syncer    ports.MailboxSyncer
	Seeder    ports.MailboxSeeder
}

// newModel wires the starting UI state from injected dependencies plus the
// config-derived theme, keybindings, and compose inputs.
func newModel(deps Dependencies) Model {
	cfg := deps.Config

	bindings := defaultKeyMap()
	styles := DefaultStyles()
	if cfg != nil {
		bindings = keyMapFromConfig(cfg.Keybindings)
		styles = StylesFromTheme(cfg.Theme)
	}

	m := Model{
		config:           cfg,
		state:            stateSidebar,
		keys:             bindings,
		help:             help.New(),
		styles:           styles,
		sidebarItems:     nil,
		sidebarCursor:    0,
		service:          deps.Service,
		store:            deps.Store,
		assistant:        deps.Assistant,
		syncer:           deps.Syncer,
		seeder:           deps.Seeder,
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
		accountNames:     nil,
		accountEmails:    map[string]string{},
		defaultFrom:      "",
		defaultAcctID:    "",
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
		aiPromptInput: func() textinput.Model {
			input := textinput.New()
			input.Prompt = "» "
			input.Placeholder = "e.g. politely accept and ask for the agenda"
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
	m.applyAccountsFromConfig(cfg)
	if cfg == nil || len(cfg.Accounts) == 0 {
		// First run: walk the user through adding an account.
		m.enterSetupState()
	}
	return m
}

// applyAccountsFromConfig refreshes account-derived UI state (sidebar rows,
// account names, default sender) after the config changes.
func (m *Model) applyAccountsFromConfig(cfg *config.Config) {
	items := []string{"Inbox", "Sent", "Drafts", "Archive", "Trash", "Spam"}
	names := []string{}
	emails := map[string]string{}
	if cfg != nil {
		if len(cfg.Accounts) > 0 {
			m.defaultAcctID = cfg.Accounts[0].Name
			m.defaultFrom = cfg.Accounts[0].Email
		}
		for _, acc := range cfg.Accounts {
			names = append(names, acc.Name)
			emails[acc.Name] = acc.Email
		}
		if len(cfg.Accounts) > 0 {
			items = append(items, "", "Accounts:")
			for _, acc := range cfg.Accounts {
				items = append(items, fmt.Sprintf("  %s", acc.Name))
			}
		}
	}
	m.sidebarItems = items
	m.accountNames = names
	m.accountEmails = emails
	if m.sidebarCursor >= len(m.sidebarItems) {
		m.sidebarCursor = 0
	}
}

// applyDemoAccount sets an in-memory demo account for browsing sample mail.
// Nothing is written to the config file, so a later refresh reloads locally
// instead of attempting a real IMAP sync.
func (m *Model) applyDemoAccount() {
	name, email := models.DemoAccountName, models.DemoAccountEmail
	m.sidebarItems = []string{"Inbox", "Sent", "Drafts", "Archive", "Trash", "Spam", "", "Accounts:", "  " + name}
	m.accountNames = []string{name}
	m.accountEmails = map[string]string{name: email}
	m.defaultAcctID = name
	m.defaultFrom = email
	if m.sidebarCursor >= len(m.sidebarItems) {
		m.sidebarCursor = 0
	}
}

func (m Model) Init() tea.Cmd {
	// With real accounts, pull fresh mail from the server on launch (this also
	// purges any leftover demo data), so the inbox is populated without a manual
	// refresh — e.g. right after clearing the local store. Otherwise (demo mode /
	// no accounts) just read whatever is local.
	if m.canSyncAccounts() {
		return tea.Batch(m.loadingTickCmd(), m.syncAccountsCmd())
	}
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
