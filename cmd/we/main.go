// Command we is the reference toolchain's one command-line entry (chapter
// 21: every toolchain operation is `we <subcommand> [path] [options]`).
package main

import (
	"os"

	"github.com/ltlvtao/welang/internal/cli"
)

func main() {
	os.Exit(cli.Run(os.Args[1:], os.Stdout, os.Stderr))
}
