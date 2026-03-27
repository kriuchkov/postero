package tui

import (
	"strings"

	"github.com/charmbracelet/bubbles/key"

	"github.com/kriuchkov/postero/internal/config"
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
	Repeat    key.Binding
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
	return []key.Binding{k.Up, k.Down, k.Left, k.Right, k.Enter, k.Search, k.Quit}
}

func (k keyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Up, k.Down, k.Left, k.Right},
		{k.Enter, k.Search, k.Refresh},
		{k.Undo, k.Repeat},
		{k.HalfUp, k.HalfDown, k.Top, k.Bottom},
		{k.Compose, k.Reply, k.ReplyAll, k.Forward},
		{k.Archive, k.Spam, k.Delete},
		{k.Edit, k.SaveDraft, k.Send},
		{k.Quit},
	}
}

// defaultKeyMap keeps the builtin bindings compact around the main vim-like navigation flow.
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
		Repeat:    key.NewBinding(key.WithKeys("."), key.WithHelp(".", "repeat")),
		PageUp:    key.NewBinding(key.WithKeys("pgup"), key.WithHelp("pgup", "page up")),
		PageDown:  key.NewBinding(key.WithKeys("pgdown"), key.WithHelp("pgdn", "page down")),
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
		Delete:    key.NewBinding(key.WithKeys("d", "delete"), key.WithHelp("d", "delete")),
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
	bindings.Delete = newBinding(cfg.Delete, []string{"d", "delete"}, "delete")
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
