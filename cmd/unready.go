package cmd

import (
	"github.com/spf13/cobra"
	"github.com/tedkulp/tix/internal/logger"
	"github.com/tedkulp/tix/internal/utils"
)

var (
	unreadyLabel string
)

var unreadyCmd = &cobra.Command{
	Use:   "unready",
	Short: "Mark an issue as not ready by removing a predefined label",
	Long: `Mark an issue as not ready by removing a predefined label from it.
The command will automatically determine the current project and issue number from the branch name.

You can configure the default ready label globally in your .tix.yaml file:
  ready_label: "ready for review"

Or configure it per repository:
  repositories:
    - name: myrepo
      ready_label: "review-ready"

You can also override the label for a specific use:
  tix unready --label "needs-review"`,
	RunE: func(cmd *cobra.Command, args []string) error {
		logger.Debug("Starting unready command")
		return utils.HandleLabelOperation(utils.RemoveLabel, unreadyLabel)
	},
}

func init() {
	rootCmd.AddCommand(unreadyCmd)

	// Add flags
	unreadyCmd.Flags().StringVarP(&unreadyLabel, "label", "l", "", "Override the default ready label to remove")
}
