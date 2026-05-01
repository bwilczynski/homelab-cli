package main

import (
	"os"

	"github.com/bwilczynski/hlctl/internal/cli"
)

func main() {
	if err := cli.Execute(); err != nil {
		os.Exit(1)
	}
}
