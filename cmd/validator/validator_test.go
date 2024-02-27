package cmd

// import (
// 	"flag"
// 	"fmt"
// 	"os"
// 	"testing"
// )

// func Test_flags(t *testing.T) {
// 	// We manipuate the Args to set them up for the testcases
// 	// After this test we restore the initial args
// 	oldArgs := os.Args
// 	defer func() { os.Args = oldArgs }()
// 	cases := []struct {
// 		Name         string
// 		Args         []string
// 		ExpectedExit int
// 	}{
// 		{"blank", []string{}, 0},
// 		{"negative depth set", []string{"-depth=-1", "."}, 1},
// 		{"depth set", []string{"-depth=1", "."}, 0},
// 		{"flags set, wrong reporter", []string{"--exclude-dirs=subdir", "--reporter=wrong", "."}, 1},
// 		{"flags set, json reporter", []string{"--exclude-dirs=subdir", "--reporter=json", "."}, 0},
// 		{"flags set, junit reported", []string{"--exclude-dirs=subdir", "--reporter=junit", "."}, 0},
// 		{"bad path", []string{"/path/does/not/exit"}, 1},
// 		{"exclude file types set", []string{"--exclude-file-types=json", "."}, 0},
// 		{"multiple paths", []string{"../../test/fixtures/subdir/good.json", "../../test/fixtures/good.json"}, 0},
// 		{"version", []string{"--version"}, 0},
// 		{"output set", []string{"--output=../../test/output", "--reporter=json", "."}, 0},
// 		{"empty string output set", []string{"--output", "", "--reporter", "json", "."}, 0},
// 		{"wrong output set", []string{"--output", "/path/not/exist", "--reporter", "json", "."}, 1},
// 		{"incorrect group", []string{"-groupby=badgroup", "."}, 1},
// 		{"correct group", []string{"-groupby=directory", "."}, 0},
// 	}
// 	for _, tc := range cases {
// 		// this call is required because otherwise flags panics,
// 		// if args are set between flag.Parse call
// 		fmt.Printf("Testing args: %v = %v\n", tc.Name, tc.Args)
// 		flag.CommandLine = flag.NewFlagSet(tc.Name, flag.ExitOnError)
// 		// we need a value to set Args[0] to cause flag begins parsing at Args[1]
// 		os.Args = append([]string{tc.Name}, tc.Args...)
// 		actualExit := mainInit()
// 		if tc.ExpectedExit != actualExit {
// 			t.Errorf("Wrong exit code, expected: %v, got: %v", tc.ExpectedExit, actualExit)
// 		}
// 	}
// }
