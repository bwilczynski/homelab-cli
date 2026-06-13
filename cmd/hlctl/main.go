package main

import (
	"os"

	"github.com/bwilczynski/hlctl/internal/cli"
	"github.com/bwilczynski/hlctl/internal/cli/cmdutil"
	"github.com/spf13/pflag"
)

var version = "dev"

func main() {
	var apiURL, outputFmt string
	pflag.StringVarP(&outputFmt, "output", "o", "table", "Output format: table or json")
	pflag.StringVar(&apiURL, "api-url", "", "Override API base URL")

	f := cmdutil.NewFactory(version, &apiURL, &outputFmt)
	root := cli.NewRootCmd(f)
	root.PersistentFlags().AddFlagSet(pflag.CommandLine)
	if err := root.Execute(); err != nil {
		os.Exit(1)
	}
}
