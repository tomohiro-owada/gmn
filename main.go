package main

import (
	"fmt"
	"os"

	"github.com/k-sub1995/g/cmd"
)

// version is set via ldflags at build time
var version = "dev"

func main() {
	cmd.SetVersion(version)
	if err := cmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
