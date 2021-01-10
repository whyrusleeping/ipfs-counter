package main

import (
	"os"

	"github.com/urfave/cli/v2"
)

func main() {
	app := &cli.App{
		Name:   "ipfs-crawler",
		Usage:  "Spider nodes in the IPFS network",
		Flags:  crawlFlags,
		Action: crawl,
	}

	err := app.Run(os.Args)
	if err != nil {
		panic(err)
	}
}
