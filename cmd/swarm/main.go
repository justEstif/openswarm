package main

import (
	"fmt"
	"os"
)

// Populated by goreleaser ldflags.
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "version" {
		fmt.Printf("swarm %s (%s, %s)\n", version, commit, date)
		return
	}
	_, _ = fmt.Fprintln(os.Stderr, "swarm: not yet implemented — see docs/ARCHITECTURE.md")
	os.Exit(1)
}
