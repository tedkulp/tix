package cmd

import (
	"os"

	"github.com/spf13/cobra"
	"github.com/tedkulp/tix/internal/logger"
)

// Flag variables
var (
	verbose bool
)

var rootCmd = &cobra.Command{
	Use:   "tix",
	Short: "A CLI tool for creating tickets and branches",
	Long: `Tix is a CLI tool that helps you create tickets and branches
in your Git repositories, with support for both GitHub and GitLab.`,
	// Only initialize logging for actual command execution (not help)
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		// Skip logging initialization for help and completion commands
		if cmd.Name() == "help" || cmd.Name() == "completion" {
			return nil
		}

		// Initialize logger with verbose flag
		logger.InitLogger(verbose)

		if verbose {
			logger.Info("Verbose logging enabled")
			logger.Debug("Debug logging is active")
		} else {
			logger.Info("Running in normal mode")
		}

		return nil
	},
	// Add a Run function to show help if no subcommand is provided
	Run: func(cmd *cobra.Command, args []string) {
		// Initialize logger for root command when no subcommand is specified
		if !cmd.HasSubCommands() {
			logger.InitLogger(verbose)

			if verbose {
				logger.Info("Verbose logging enabled")
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
	return rootCmd.Execute()
}

func init() {
	// Here you will define your flags and configuration settings.
	// Cobra supports persistent flags, which, if defined here,
	// will be global for your application.
	rootCmd.PersistentFlags().StringP("config", "c", "~/.tix.yml", "config file (default is $HOME/.tix.yml)")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "enable verbose logging")
}
