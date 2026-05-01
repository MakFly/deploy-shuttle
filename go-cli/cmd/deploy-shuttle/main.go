package main

import (
	"os"

	"github.com/MakFly/deploy-shuttle/go-cli/internal/cli"
)

func main() {
	if err := cli.NewRootCommand().Execute(); err != nil {
		os.Exit(1)
	}
}
