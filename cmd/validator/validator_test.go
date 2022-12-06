package main

import (
	"flag"
	"fmt"
	"os"
	"testing"
)

func Test_flags(t *testing.T) {
	// We manipuate the Args to set them up for the testcases
	// After this test we restore the initial args
	oldArgs := os.Args
	defer func() { os.Args = oldArgs }()
	cases := []struct {
		Name         string
		Args         []string
		ExpectedExit int
	}{
		{"flags set", []string{"--search-path=.",
			"--exclude-dirs=subdir"}, 0},
		{"blank", []string{}, 1},
	}
	for _, tc := range cases {
		// this call is required because otherwise flags panics,
		// if args are set between flag.Parse call
		fmt.Printf("Testing args: %v = %v\n", tc.Name, tc.Args)
		flag.CommandLine = flag.NewFlagSet(tc.Name, flag.ExitOnError)
		// we need a value to set Args[0] to cause flag begins parsing at Args[1]
		os.Args = append([]string{tc.Name}, tc.Args...)
		_, _, actualExit := getFlags()
		if tc.ExpectedExit != actualExit {
			t.Errorf("Wrong exit code, expected: %v, got: %v", tc.ExpectedExit, actualExit)
		}
	}
}

func Test_getCLIValues(t *testing.T) {
	searchPathVal := "/fake/test/path"
	excludeDirsVal := "subdir,subdir2"

	// create pointers to the values
	searchPathPtr := &searchPathVal
	excludeDirsPtr := &excludeDirsVal
	searchPath, excludeDirs := getCLIValues(searchPathPtr, excludeDirsPtr)

	if searchPath != searchPathVal {
		t.Errorf("Search path does not match original value")
	}

	if len(excludeDirs) != 2 {
		t.Errorf("Exclude dirs were not properly split")
	}
}

func Test_mainInit(t *testing.T) {
	oldArgs := os.Args
	defer func() { os.Args = oldArgs }()
	// this call is required because otherwise flags panics,
	// if args are set between flag.Parse call
	flag.CommandLine = flag.NewFlagSet("test", flag.ExitOnError)
	// we need a value to set Args[0] to cause flag begins parsing at Args[1]
	os.Args = append(
		[]string{"test"},
		[]string{"--search-path=../../test", "--exclude-dirs=subdir,subdir2"}...,
	)
	exitCode := mainInit()
	if exitCode != 0 {
		t.Errorf("Main init returned non zero")
	}
}

func Test_mainInitBadFlags(t *testing.T) {
	oldArgs := os.Args
	defer func() { os.Args = oldArgs }()
	// this call is required because otherwise flags panics,
	// if args are set between flag.Parse call
	flag.CommandLine = flag.NewFlagSet("test2", flag.ExitOnError)
	// we need a value to set Args[0] to cause flag begins parsing at Args[1]
	os.Args = append(
		[]string{"test2"},
		[]string{"--search-path="}...,
	)
	exitCode := mainInit()
	if exitCode == 0 {
		t.Errorf("Main init returned zero")
	}
}

func Test_mainInitBadSearchPath(t *testing.T) {
	oldArgs := os.Args
	defer func() { os.Args = oldArgs }()
	// this call is required because otherwise flags panics,
	// if args are set between flag.Parse call
	flag.CommandLine = flag.NewFlagSet("test2", flag.ExitOnError)
	// we need a value to set Args[0] to cause flag begins parsing at Args[1]
	os.Args = append(
		[]string{"test2"},
		[]string{"--search-path=/does/not/exist"}...,
	)
	exitCode := mainInit()
	if exitCode == 0 {
		t.Errorf("Main init returned zero")
	}
}
