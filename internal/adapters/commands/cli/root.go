package cli

import (
	"github.com/spf13/cobra"

	"github.com/kriuchkov/postero/internal/adapters/ui/tui"
	appcore "github.com/kriuchkov/postero/internal/app"
	"github.com/kriuchkov/postero/internal/core/ports"
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "pstr",
	Short: "Postero - Terminal-First Email Client",
	Long: `Postero (pstr) is a modern open-source terminal email client designed for productivity.
Built from the ground up for developers, engineers, and command-line aficionados.`,
	Version: "1.0.0",
	RunE: func(_ *cobra.Command, _ []string) error {
		return tui.Run(buildTUIDependencies())
	},
}

// buildTUIDependencies is the composition root for the TUI: it wires config,
// services, and mailbox capabilities and hands them to the UI adapter as ports,
// so the TUI never depends on the wiring package.
func buildTUIDependencies() tui.Dependencies {
	cfg, _ := appcore.LoadConfig()
	service, store, _, _ := appcore.NewMessageServiceWithStore()

	var assistant ports.DraftAssistant
	if cfg != nil {
		if a, err := appcore.NewDraftAssistantWithConfig(cfg); err == nil {
			assistant = a
		}
	}

	var (
		syncer ports.MailboxSyncer
		seeder ports.MailboxSeeder
	)
	if store != nil {
		syncer = appcore.NewMailboxSyncer(store, cfg)
		seeder = appcore.NewMailboxSeeder(store)
	}

	return tui.Dependencies{
		Config:    cfg,
		Service:   service,
		Store:     store,
		Assistant: assistant,
		Syncer:    syncer,
		Seeder:    seeder,
	}
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
