package main

import (
	"fmt"
	"log"

	"github.com/Boeing/config-file-validator/v3/pkg/cli"
)

func main() {
	if err := cli.Run(); err != nil {
		log.Fatal(err)
	}
}