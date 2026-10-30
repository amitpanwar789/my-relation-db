// This file starts the SQL command-line application.
// It connects keyboard input and terminal output to the shell.
// It enables prompts for interactive input and reports input/output failures.
// The shell creates in-memory tables, inserts rows, and displays query results.

package main

import (
	"fmt"
	"os"

	"github.com/amitpanwar789/my-relation-db/internal/shell"
)

func main() {
	info, err := os.Stdin.Stat()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	interactive := info.Mode()&os.ModeCharDevice != 0
	if err := shell.Run(os.Stdin, os.Stdout, interactive); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
