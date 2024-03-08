package cmd

import (
	"fmt"
	"os"

	validator "github.com/Boeing/config-file-validator/cmd/validator"
	"github.com/spf13/cobra"
)

// rootCmd command configuration and setup
var rootCmd = &cobra.Command{
	Use:   "validator",
	Short: "Cross Platform tool to validate configuration files",
	Run: func(cmd *cobra.Command, args []string) {
		os.Exit(validator.ExecRoot(cmd))
	},
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
