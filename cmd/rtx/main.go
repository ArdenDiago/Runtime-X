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
		fmt.Fprintf(os.Stderr, "usage: rtx [flags] <subcommand> [args...]\n\n")
		fmt.Fprintf(os.Stderr, "subcommands:\n")
		fmt.Fprintf(os.Stderr, "  run <command> [args...]  run a process and forward signals\n")
		fmt.Fprintf(os.Stderr, "  serve [-port <n>]        start REST API and serve frontend\n\n")
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
		return cmdRun(args[1:])
	case "serve":
		return cmdServe(args[1:])
	default:
		fmt.Fprintf(os.Stderr, "[rtx] unknown subcommand: %s\n", args[0])
		flag.Usage()
		return 1
	}
}

// cmdRun implements `rtx run <command> [args...]`.
// CLI-01: spawn command and forward signals; CLI-02: PID logged by process.Run().
func cmdRun(args []string) int {
	fs := flag.NewFlagSet("run", flag.ContinueOnError)
	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "usage: rtx run <command> [args...]\n")
	}
	if err := fs.Parse(args); err != nil {
		return 1
	}
	remaining := fs.Args()
	if len(remaining) == 0 {
		fmt.Fprintf(os.Stderr, "[rtx] error: 'run' requires a command\n")
		fs.Usage()
		return 1
	}
	return process.Run(remaining[0], remaining[1:])
}
