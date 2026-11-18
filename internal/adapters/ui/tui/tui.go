package tui

import (
	tea "github.com/charmbracelet/bubbletea"
)

// NewModel builds the root TUI model from injected dependencies.
func NewModel(deps Dependencies) Model {
	return newModel(deps)
}

// Run starts the Bubble Tea program with the given dependencies.
func Run(deps Dependencies) error {
	p := tea.NewProgram(NewModel(deps), tea.WithAltScreen())
	_, err := p.Run()
	return err
}
