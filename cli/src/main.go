package main

import (
	"fmt"
	"os"

	"github.com/nexturecorp/aico/src/cmd"
)

// version is set at release time via -ldflags "-X main.version=<tag>".
var version = "dev"

func main() {
	cmd.SetVersion(version)
	if err := cmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
