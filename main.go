package main

import (
	"fmt"
	"os"

	"github.com/urfave/cli/v2"
)

// Version is the running version. It can be over-ridden by build tags.
var Version string = "unknown"

func main() {
	app := &cli.App{
		Name:    "ipfs-crawler",
		Usage:   "Spider nodes in the IPFS network",
		Flags:   crawlFlags,
		Action:  crawl,
		Version: Version,
	}

	err := app.Run(os.Args)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
}
