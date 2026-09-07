package tui

import (
	"gomodtui/internal/tui"

	tea "charm.land/bubbletea/v2"
	"github.com/spf13/cobra"
)

// NewTuiCmd creates the tui subcommand.
func NewTuiCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "tui",
		Short: "Launch browser-like TUI",
		Long:  `Launch an interactive browser-like TUI to search pkg.go.dev with markdown rendering and hjkl/mouse navigation.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			m := tui.NewModel()
			p := tea.NewProgram(m)
			_, err := p.Run()
			return err
		},
	}
	return cmd
}
