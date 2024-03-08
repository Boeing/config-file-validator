package cmd

import (
	validator "github.com/Boeing/config-file-validator/cmd/validator"
	"github.com/spf13/cobra"
)

func CmdFlags(cmd *cobra.Command) {
	cmd.PersistentFlags().
		IntVar(&validator.Flags.Depth, "depth", 0, "Depth of recursion for the provided search paths. Set depth to 0 to disable recursive path traversal.")
	cmd.PersistentFlags().
		StringVar(&validator.Flags.ExcludeDirs, "exclude-dirs", "", "Subdirectories to exclude when searching for configuration files")
	cmd.PersistentFlags().
		StringVar(&validator.Flags.ExcludeFileTypes, "exclude-file-types", "", "A comma separated list of file types to ignore")
	cmd.PersistentFlags().StringVar(&validator.Flags.Output, "output", "", "Destination to a file to output results")
	cmd.PersistentFlags().
		StringVar(&validator.Flags.ReportType, "reporter", "standard", "Format of the printed report. Options are standard and json")
	cmd.PersistentFlags().
		StringVar(&validator.Flags.GroupOutput, "groupby", "", "Group output by filetype, directory, pass-fail. Supported for Standard and JSON reports")
	cmd.PersistentFlags().
		StringVar(&validator.Flags.SearchPath, "search_path", ".", "search_path: The search path on the filesystem for configuration files. Defaults to the current working directory if no search_path provided.")
}

func init() {
	CmdFlags(rootCmd)
}
