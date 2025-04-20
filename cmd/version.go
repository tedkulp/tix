package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/tedkulp/tix/internal/version"
)

// versionCmd represents the version command
var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Display version information",
	Long:  `Display the current version of tix, including the commit hash and build date.`,
	Run: func(cmd *cobra.Command, args []string) {
		// Get verbose flag
		verbose, _ := cmd.Flags().GetBool("verbose")

		if verbose {
			fmt.Println(version.GetFullVersion())
		} else {
			fmt.Println(version.GetVersion())
		}
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
