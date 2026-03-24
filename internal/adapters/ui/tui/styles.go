package tui

import (
	"github.com/charmbracelet/lipgloss"
	"github.com/kriuchkov/postero/internal/config"
)

type Palette struct {
	Primary   lipgloss.Color
	Secondary lipgloss.Color
	Text      lipgloss.Color
	SubText   lipgloss.Color
	Highlight lipgloss.Color
	Faint     lipgloss.Color
}

type Styles struct {
	Palette Palette
	Header  lipgloss.Style
	Sidebar lipgloss.Style
	List    lipgloss.Style
	Content lipgloss.Style
	Footer  lipgloss.Style
}

func paneTitleStyle(m Model, pane SessionState) lipgloss.Style {
	style := lipgloss.NewStyle().Bold(true).Foreground(m.styles.Palette.Text)
	if m.state == pane {
		style = style.Foreground(m.styles.Palette.Highlight)
	}
	return style
}

func paneSubtitleStyle(m Model, pane SessionState) lipgloss.Style {
	style := lipgloss.NewStyle().Foreground(m.styles.Palette.SubText)
	if m.state == pane {
		style = style.Foreground(m.styles.Palette.Text)
	}
	return style
}

func DefaultStyles() Styles {
	return StylesFromTheme(config.ThemeConfig{})
}

func StylesFromTheme(theme config.ThemeConfig) Styles {
	palette := Palette{
		Primary:   lipgloss.Color(defaultThemeValue(theme.Primary, "33")),
		Secondary: lipgloss.Color(defaultThemeValue(theme.Secondary, "240")),
		Text:      lipgloss.Color(defaultThemeValue(theme.Text, "252")),
		SubText:   lipgloss.Color(defaultThemeValue(theme.SubText, "245")),
		Highlight: lipgloss.Color(defaultThemeValue(theme.Highlight, "255")),
		Faint:     lipgloss.Color(defaultThemeValue(theme.Faint, "236")),
	}

	return Styles{
		Palette: palette,
		Header: lipgloss.NewStyle().
			Border(lipgloss.NormalBorder(), false, false, true, false).
			BorderForeground(palette.Faint).
			Padding(0, 1),
		Sidebar: lipgloss.NewStyle().
			Border(lipgloss.NormalBorder(), false, true, false, false).
			BorderForeground(palette.Faint).
			Padding(1, 1),
		List: lipgloss.NewStyle().
			Border(lipgloss.NormalBorder(), false, true, false, false).
			BorderForeground(palette.Faint).
			Padding(1, 1),
		Content: lipgloss.NewStyle().
			Padding(1, 2),
		Footer: lipgloss.NewStyle().
			Border(lipgloss.NormalBorder(), true, false, false, false).
			BorderForeground(palette.Faint).
			Padding(0, 1),
	}
}

func defaultThemeValue(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}
