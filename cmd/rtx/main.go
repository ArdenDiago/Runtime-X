package main

import (
	"flag"
	"fmt"
	"os"

	"runtimex/internal/process"
)

func main() {
	// EXIT-02: os.Exit only in main() so deferred cleanup in run() executes.
	os.Exit(run())
}

func run() int {
	// Global flags (before subcommand)
	verbose := flag.Bool("v", false, "verbose output")
	flag.BoolVar(verbose, "verbose", false, "verbose output")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "usage: rtx [flags] run <command> [args...]\n\n")
		fmt.Fprintf(os.Stderr, "flags:\n")
		flag.PrintDefaults()
	}
	flag.Parse()

	args := flag.Args()
	if len(args) == 0 {
		flag.Usage()
		return 1
	}

	switch args[0] {
	case "run":
		// CLI-01: rtx run <command> [args...]
		if len(args) < 2 {
			fmt.Fprintf(os.Stderr, "[rtx] error: 'run' requires a command\n")
			fmt.Fprintf(os.Stderr, "usage: rtx run <command> [args...]\n")
			return 1
		}
		// CLI-02: PID is logged inside process.Run() immediately after spawn
		return process.Run(args[1], args[2:])
	default:
		fmt.Fprintf(os.Stderr, "[rtx] unknown subcommand: %s\n", args[0])
		fmt.Fprintf(os.Stderr, "usage: rtx [flags] run <command> [args...]\n")
		return 1
	}
}
