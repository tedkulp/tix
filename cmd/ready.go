package cmd

import (
	"github.com/spf13/cobra"
	"github.com/tedkulp/tix/internal/logger"
	"github.com/tedkulp/tix/internal/utils"
)

var (
	readyLabel string
)

var readyCmd = &cobra.Command{
	Use:   "ready",
	Short: "Mark an issue as ready by adding a predefined label",
	Long: `Mark an issue as ready by adding a predefined label to it.
The command will automatically determine the current project and issue number from the branch name.

You can configure the default ready label globally in your .tix.yaml file:
  ready_label: "ready for review"

Or configure it per repository:
  repositories:
    - name: myrepo
      ready_label: "review-ready"

You can also override the label for a specific use:
  tix ready --label "needs-review"`,
	RunE: func(cmd *cobra.Command, args []string) error {
		logger.Debug("Starting ready command")
		return utils.HandleLabelOperation(utils.AddLabel, readyLabel)
	},
}

func init() {
	rootCmd.AddCommand(readyCmd)

	// Add flags
	readyCmd.Flags().StringVarP(&readyLabel, "label", "l", "", "Override the default ready label")
}
