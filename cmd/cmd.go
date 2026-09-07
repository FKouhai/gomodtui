package cmd

import (
	tea "charm.land/bubbletea/v2"
	"github.com/spf13/cobra"
	"github.com/FKouhai/gomodtui/cmd/package"
	"github.com/FKouhai/gomodtui/cmd/search"
	tuicmd "github.com/FKouhai/gomodtui/cmd/tui"
	"github.com/FKouhai/gomodtui/internal/tui"
)

// NewRootCommand creates a fresh root command instance.
func NewRootCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "gomodtui",
		Short: "TUI and cli viewer of golang libraries",
		Long:  `gomodtui: exposes different subcommands to interact with the gopkg documentation, as well as a TUI`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Default to TUI browser when no subcommand given
			if len(args) == 0 {
				m := tui.NewModel()
				p := tea.NewProgram(m)
				_, err := p.Run()
				return err
			}
			return cmd.Help()
		},
	}

	cmd.AddCommand(search.NewSearchCmd())
	cmd.AddCommand(pkg.NewPackageCmd())
	cmd.AddCommand(tuicmd.NewTuiCmd())

	return cmd
}

// Execute runs the root command.
func Execute() error {
	return NewRootCommand().Execute()
}
