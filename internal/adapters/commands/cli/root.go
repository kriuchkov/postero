package cli

import (
	"github.com/spf13/cobra"

	"github.com/kriuchkov/postero/internal/adapters/ui/tui"
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "pstr",
	Short: "Postero - Terminal-First Email Client",
	Long: `Postero (pstr) is a modern open-source terminal email client designed for productivity.
Built from the ground up for developers, engineers, and command-line aficionados.`,
	Version: "1.0.0",
	RunE: func(_ *cobra.Command, _ []string) error {
		return tui.Run()
	},
}

// Execute adds all child commands to the root command and sets flags appropriately
func Execute() {
	cobra.CheckErr(rootCmd.Execute())
}

func init() {
	rootCmd.AddCommand(syncCmd)
	rootCmd.AddCommand(searchCmd)
	rootCmd.AddCommand(composeCmd)
	rootCmd.AddCommand(replyCmd)
	rootCmd.AddCommand(forwardCmd)
	rootCmd.AddCommand(listCmd)
	rootCmd.AddCommand(authCmd)
	rootCmd.AddCommand(configCmd)
}
