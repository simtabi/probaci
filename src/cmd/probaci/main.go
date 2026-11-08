// Command probaci proves your CI pipeline before you push: it runs the same
// checks your CI runs, locally, with every tool brokered through a container.
package main

import (
	"fmt"
	"os"

	"github.com/simtabi/probaci/internal/cli"
)

func main() {
	if err := cli.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(cli.ExitCode(err))
	}
}
