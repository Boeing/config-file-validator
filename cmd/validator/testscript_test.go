package main

import (
	"os"
	"testing"

	"github.com/rogpeppe/go-internal/testscript"
)

func validatorCmd() {
	os.Exit(mainInit())
}

func TestMain(m *testing.M) {
	testscript.Main(m, map[string]func(){
		"validator": validatorCmd,
	})
}

func TestScript(t *testing.T) {
	testscript.Run(t, testscript.Params{
		Dir: "testdata",
	})
}
