package cmd

import (
	"github.com/spf13/cobra"
	"github.com/tedkulp/tix/internal/logger"
	"github.com/tedkulp/tix/internal/utils"
)

var (
	unreadyLabel      string
	unreadyStatus     string
	unreadyLabelToAdd string
)

var unreadyCmd = &cobra.Command{
	Use:   "unready",
	Short: "Mark an issue as not ready by removing ready label and optionally updating status",
	Long: `Mark an issue as not ready by removing the ready label from it and optionally updating its status.
The command will automatically determine the current project and issue number from the branch name.

You can configure the default labels and status globally in your .tix.yaml file:
  ready_label: "ready for review"
  unready_label: "needs-work"     # Optional: label to add when marking as unready
  unready_status: "opened"        # GitLab only: status to set when marking as unready

Or configure it per repository:
  repositories:
    - name: myrepo
      ready_label: "review-ready"
      unready_label: "work-needed"
      unready_status: "opened"

You can also override the labels and status for a specific use:
  tix unready --label "needs-review"                    # Override ready label to remove
  tix unready --unready-label "work-needed"             # Override unready label to add
  tix unready --status "opened"                         # Override unready status
  tix unready --label "ready" --unready-label "blocked" # Override both

Note: Status updates are only supported for GitLab issues and will be silently ignored for GitHub.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		logger.Debug("Starting unready command")
		return utils.HandleLabelAndStatusOperationWithUnready(utils.RemoveLabel, unreadyLabel, unreadyStatus, unreadyLabelToAdd)
	},
}

func init() {
	rootCmd.AddCommand(unreadyCmd)

	// Add flags
	unreadyCmd.Flags().StringVarP(&unreadyLabel, "label", "l", "", "Override the default ready label to remove")
	unreadyCmd.Flags().StringVarP(&unreadyLabelToAdd, "unready-label", "u", "", "Override the default unready label to add")
	unreadyCmd.Flags().StringVarP(&unreadyStatus, "status", "s", "", "Override the default unready status (GitLab only)")
}
