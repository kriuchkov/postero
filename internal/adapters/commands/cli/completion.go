package cli

// Shell-completion wiring. Cobra already ships the `pstr completion
// <bash|zsh|fish|powershell>` generator; this file adds the dynamic parts —
// message IDs, account names, and enum flag values — so completing an argument
// suggests real data from the local store instead of file names.

import (
	"context"
	"strings"

	"github.com/spf13/cobra"

	appcore "github.com/kriuchkov/postero/internal/app"
)

// completionListLimit bounds how many recent messages a single completion
// request loads: completions must return instantly, and a shell menu longer
// than this is unusable anyway.
const completionListLimit = 50

// completeMessageIDs suggests IDs of recent messages, with the subject and
// sender as the menu description. Completion runs in a fresh process on every
// TAB press, so any failure degrades to "no suggestions" — never an error.
func completeMessageIDs(_ *cobra.Command, _ []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	// Completion must be side-effect free: opening the repository creates the
	// data directory, and without a configured data path that would drop a
	// ./.postero folder into whatever directory the shell happens to be in.
	if cfg, err := appcore.LoadConfig(); err != nil || cfg == nil || strings.TrimSpace(cfg.DataPath) == "" {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	repo, _, err := appcore.NewMessageRepository()
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	messages, err := repo.List(context.Background(), completionListLimit, 0)
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	completions := make([]string, 0, len(messages))
	for _, msg := range messages {
		if msg == nil || !strings.HasPrefix(msg.ID, toComplete) {
			continue
		}
		subject := strings.TrimSpace(msg.Subject)
		if subject == "" {
			subject = "(no subject)"
		}
		completions = append(completions, msg.ID+"\t"+completionText(subject+" — "+msg.From))
	}
	return completions, cobra.ShellCompDirectiveNoFileComp
}

// completeAccounts suggests configured account names, with the email as the
// menu description.
func completeAccounts(_ *cobra.Command, _ []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	cfg, err := appcore.LoadConfig()
	if err != nil || cfg == nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	completions := make([]string, 0, len(cfg.Accounts))
	for _, account := range cfg.Accounts {
		if !strings.HasPrefix(account.Name, toComplete) {
			continue
		}
		completions = append(completions, account.Name+"\t"+completionText(account.Email))
	}
	return completions, cobra.ShellCompDirectiveNoFileComp
}

// completionText sanitizes a free-form string for use as a completion
// description: tabs and newlines are the cobra wire format's separators.
func completionText(value string) string {
	return strings.Join(strings.Fields(value), " ")
}

// staticCompletion returns a completion function over a fixed value set
// (enum-style flags).
func staticCompletion(values ...string) cobra.CompletionFunc {
	return cobra.FixedCompletions(values, cobra.ShellCompDirectiveNoFileComp)
}

// registerCompletions wires the dynamic completions onto the commands. It runs
// from Execute() rather than an init(): flag completion can only be registered
// after every file's init() has defined its flags, and init order across files
// is alphabetical.
func registerCompletions() {
	// Commands whose positional arguments are message IDs.
	for _, cmd := range []*cobra.Command{
		showCmd, readCmd, starCmd, archiveCmd, spamCmd, trashCmd, deleteCmd,
		replyCmd, replyAICmd, forwardCmd,
	} {
		cmd.ValidArgsFunction = completeMessageIDs
	}

	// Commands with an --account flag.
	for _, cmd := range []*cobra.Command{
		composeCmd, composeAICmd, forwardCmd, listCmd,
		replyCmd, replyAICmd, searchCmd, syncCmd,
	} {
		_ = cmd.RegisterFlagCompletionFunc("account", completeAccounts)
	}

	_ = listCmd.RegisterFlagCompletionFunc("mailbox", staticCompletion(
		"inbox", "all", "archive", "drafts", "sent", "spam", "trash", "flagged",
	))
	_ = listCmd.RegisterFlagCompletionFunc("format", staticCompletion("text", "json"))
}
