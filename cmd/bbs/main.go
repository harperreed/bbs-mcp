// ABOUTME: CLI entry point for bbs
// ABOUTME: Initializes and executes root command

package main

import (
	"fmt"
	"os"
)

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func executeRoot() error {
	defer closeGlobalStore()
	return rootCmd.Execute()
}

func main() {
	if err := executeRoot(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
