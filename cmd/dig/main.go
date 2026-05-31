// Command dig is the CLI entry point. It is intentionally thin: all logic
// lives in internal/cli and the packages it calls.
package main

import (
	"fmt"
	"os"

	"github.com/bntvllnt/dig/internal/cli"
)

func main() {
	if err := cli.NewRoot().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "dig: "+err.Error())
		os.Exit(1)
	}
}
