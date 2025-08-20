package cmd

import (
	"github.com/spf13/cobra"
	"github.com/tedkulp/tix/internal/logger"
	"github.com/tedkulp/tix/internal/utils"
)

var (
	readyLabel  string
	readyStatus string
)

var readyCmd = &cobra.Command{
	Use:   "ready",
	Short: "Mark an issue as ready by adding a predefined label and optionally updating status",
	Long: `Mark an issue as ready by adding a predefined label to it and optionally updating its status.
The command will automatically determine the current project and issue number from the branch name.

You can configure the default ready label and status globally in your .tix.yaml file:
  ready_label: "ready for review"
  ready_status: "in_progress"

Or configure it per repository:
  repositories:
    - name: myrepo
      ready_label: "review-ready"
      ready_status: "in_progress"

You can also override the label and status for a specific use:
  tix ready --label "needs-review" --status "ready"

Note: Status updates are only supported for GitLab issues and will be silently ignored for GitHub.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		logger.Debug("Starting ready command")
		return utils.HandleLabelAndStatusOperation(utils.AddLabel, readyLabel, readyStatus)
	},
}

func init() {
	rootCmd.AddCommand(readyCmd)

	// Add flags
	readyCmd.Flags().StringVarP(&readyLabel, "label", "l", "", "Override the default ready label")
	readyCmd.Flags().StringVarP(&readyStatus, "status", "s", "", "Override the default ready status (GitLab only)")
}
