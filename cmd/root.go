package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/tedkulp/tix/internal/logger"
)

// Flag variables
var (
	verboseCount int
)

var rootCmd = &cobra.Command{
	Use:   "tix",
	Short: "A CLI tool for creating tickets and branches",
	Long: `Tix is a CLI tool that helps you create tickets and branches
in your Git repositories, with support for both GitHub and GitLab.`,
	// Silence usage and errors output to provide cleaner error messages
	SilenceUsage:  true, // Don't display usage on error
	SilenceErrors: true, // Let us handle the errors
	// Only initialize logging for actual command execution (not help)
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		// Skip logging initialization for help and completion commands
		if cmd.Name() == "help" || cmd.Name() == "completion" {
			return nil
		}

		// Initialize logger with verbose count
		logger.InitLogger(verboseCount)

		switch verboseCount {
		case 0:
			// WARN level - no startup message needed
		case 1:
			logger.Info("Info logging enabled (-v)")
		default:
			logger.Info("Debug logging enabled (-vv)")
			logger.Debug("Debug logging is active")
		}

		return nil
	},
	// Add a Run function to show help if no subcommand is provided
	Run: func(cmd *cobra.Command, args []string) {
		// Initialize logger for root command when no subcommand is specified
		if !cmd.HasSubCommands() {
			logger.InitLogger(verboseCount)

			switch verboseCount {
			case 0:
				// WARN level - no startup message needed
			case 1:
				logger.Info("Info logging enabled (-v)")
			default:
				logger.Info("Debug logging enabled (-vv)")
				logger.Debug("Debug logging is active")
			}
		}

		// If no args, show the help
		if len(args) == 0 {
			err := cmd.Help()
			if err != nil {
				logger.Error("Failed to show help", err)
			}
			os.Exit(0)
		}
	},
}

func Execute() error {
	err := rootCmd.Execute()
	// Handle errors here instead of Cobra's default handling
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %s\n", err.Error())
		return err
	}
	return nil
}

func init() {
	// Here you will define your flags and configuration settings.
	// Cobra supports persistent flags, which, if defined here,
	// will be global for your application.
	rootCmd.PersistentFlags().StringP("config", "c", "~/.tix.yml", "config file (default is $HOME/.tix.yml)")
	rootCmd.PersistentFlags().CountVarP(&verboseCount, "verbose", "v", "increase verbosity: -v for INFO, -vv for DEBUG (default: WARN)")
}
