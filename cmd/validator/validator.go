/*
Validator recusively scans a directory to search for configuration files and
validates them using the go package for each configuration type.

Currently json, yaml, toml, and xml configuration file types are supported.

Usage:

    validator [flags]

The flags are:
    -search-path string
		The search path for configuration files
    -exclude-dirs string
    	Subdirectories to exclude when searching for configuration files
*/

package main

import (
	"flag"
	"fmt"
	"github.com/Boeing/config-file-validator/pkg/cli"
	"log"
	"os"
	"strings"
)

// Parses, validates, and returns the flags
// flag.String returns a pointer
// If a required parameter is missing the help
// output will be displayed and the function
// will return with exit = 1
func getFlags() (*string, *string, int) {
	searchPathPtr := flag.String("search-path", "", "The search path for configuration files")
	excludeDirsPtr := flag.String("exclude-dirs", "", "Subdirectories to exclude when searching for configuration files")
	flag.Parse()

	exit := 0

	if *searchPathPtr == "" {
		fmt.Println("Missing required Parameter. Showing help: ")
		flag.PrintDefaults()
		exit = 1
		return nil, nil, exit
	}

	return searchPathPtr, excludeDirsPtr, exit
}

// Takes the flag values as function arguments and
// transforms them to appropriate types for initializing
// the CLI.
// searchPathPtr is changed from a pointer since CLI.init()
// requires a non-pointer value
// excludeDirsPtr is changed from a comma separated list
// of directories to an array of strings
func getCLIValues(searchPathPtr, excludeDirsPtr *string) (string, []string) {
	searchPath := *searchPathPtr
	// since the exclude dirs are a comma separated string
	// it needs to be split into a slice of strings
	excludeDirs := strings.Split(*excludeDirsPtr, ",")

	return searchPath, excludeDirs
}

func mainInit() int {
	searchPathPtr, excludeDirsPtr, exit := getFlags()
	if exit != 0 {
		return exit
	}

	searchPath, excludeDirs := getCLIValues(searchPathPtr, excludeDirsPtr)

	// Create an instance of the CLI using the
	// searchPath and excludeDirs values provided
	// by the command line arguments
	cli := cli.Init(searchPath, excludeDirs)
	exitStatus, err := cli.Run()
	if err != nil {
		log.Printf("An error occured during CLI execution: %v", err)
	}

	return exitStatus
}

func main() {
	os.Exit(mainInit())
}
