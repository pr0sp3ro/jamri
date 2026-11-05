package main

import (
	"fmt"
	"os"

	"github.com/pr0sp3ro/jamri/internal/cli"
)

func main() {
	if err := cli.Run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "jamri:", err)
		os.Exit(1)
	}
}
