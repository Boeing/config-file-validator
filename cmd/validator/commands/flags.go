package cmd

//var depthPtr = flag.Int("depth", 0, "Depth of recursion for the provided search paths. Set depth to 0 to disable recursive path traversal")
//var excludeDirsPtr = flag.String("exclude-dirs", "", "Subdirectories to exclude when searching for configuration files")

//var excludeFileTypesPtr = flag.String("exclude-file-types", "", "A comma separated list of file types to ignore")
//var outputPtr = flag.String("output", "", "Destination to a file to output results")
//var reportTypePtr = flag.String("reporter", "standard", "Format of the printed report. Options are standard and json")
//var versionPtr = flag.Bool("version", false, "Version prints the release version of validator")
//var groupOutputPtr = flag.String("groupby", "", "Group output by filetype, directory, pass-fail. Supported for Standard and JSON reports")

type ValidatorConfig struct {
	SearchPaths      []string
	Depth int
	ExcludeDirs string
	ExcludeFileTypes string
	Output string
	ReportType string
	GroupOutput string
	SearchPath string
}

var Flags ValidatorConfig

func init () {
	rootCmd.PersistentFlags().IntVar(&Flags.Depth, "depth", 0, "Depth of recursion for the provided search paths. Set depth to 0 to disable recursive path traversal. (default 0)")
	rootCmd.PersistentFlags().StringVar(&Flags.ExcludeDirs, "exclude-dirs", "", "Subdirectories to exclude when searching for configuration files")
	rootCmd.PersistentFlags().StringVar(&Flags.ExcludeFileTypes, "exclude-file-types", "", "A comma separated list of file types to ignore")
	rootCmd.PersistentFlags().StringVar(&Flags.Output, "output", "", "Destination to a file to output results")
	rootCmd.PersistentFlags().StringVar(&Flags.ReportType, "reporter", "standard", "Format of the printed report. Options are standard and json")
	rootCmd.PersistentFlags().StringVar(&Flags.GroupOutput, "groupby", "", "Group output by filetype, directory, pass-fail. Supported for Standard and JSON reports")
	rootCmd.PersistentFlags().StringVar(&Flags.SearchPath, "search_path", ".", "search_path: The search path on the filesystem for configuration files. Defaults to the current working directory if no search_path provided.")
}
