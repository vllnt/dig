// Command dig is the CLI entry point. It is intentionally thin: all logic
// lives in internal/cli and the packages it calls.
package main

import (
	"fmt"
	"os"

	"github.com/vllnt/dig/internal/cli"
)

// Build metadata, injected by GoReleaser via -ldflags at release time and
// left at their dev defaults for `go build` / `go install`.
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	cli.Version = cli.BuildInfo{Version: version, Commit: commit, Date: date}
	if err := cli.NewRoot().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "dig: "+err.Error())
		os.Exit(1)
	}
}
