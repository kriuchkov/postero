package tui

import (
	tea "github.com/charmbracelet/bubbletea"
)

func NewModel() Model {
	return initialModel()
}

func Run() error {
	p := tea.NewProgram(NewModel(), tea.WithAltScreen())
	_, err := p.Run()
	return err
}
