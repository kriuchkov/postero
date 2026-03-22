package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/cobra/doc"
)

var manCmd = &cobra.Command{
	Use:    "man [dir]",
	Short:  "Generate man pages",
	Long:   `Generate man pages for all postero commands in the specified directory.`,
	Args:   cobra.ExactArgs(1),
	Hidden: true, // Might not need to be shown in general help
	RunE: func(cmd *cobra.Command, args []string) error {
		dir := args[0]
		err := os.MkdirAll(dir, 0o700)
		if err != nil {
			return err
		}

		header := &doc.GenManHeader{
			Title:   "POSTERO",
			Section: "1",
		}

		err = doc.GenManTree(rootCmd, header, dir)
		if err != nil {
			return err
		}

		fmt.Printf("Man pages successfully generated in %s\n", dir)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(manCmd)
}
