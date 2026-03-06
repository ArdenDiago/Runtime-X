package main

import (
	"flag"
	"fmt"
	"net/http"
	"os"

	"runtimex/internal/api"
	"runtimex/internal/scheduler"
)

// cmdServe implements `rtx serve [-port <n>]`.
// It starts the REST API server backed by a new Scheduler, and serves the
// React frontend from web/dist at the root path.
func cmdServe(args []string) int {
	fs := flag.NewFlagSet("serve", flag.ContinueOnError)
	port := fs.Int("port", 8080, "port to listen on")
	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "usage: rtx serve [-port <n>]\n\n")
		fmt.Fprintf(os.Stderr, "flags:\n")
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		return 1
	}

	sched := scheduler.New()
	srv := api.NewServer(sched)

	// Build a top-level mux:
	//   /api/...   -> REST API handler (via srv.Routes())
	//   /          -> static files from web/dist
	mux := http.NewServeMux()

	// Register API routes under /api/ prefix.
	// srv.Routes() returns a CORS-wrapped handler; we strip the /api prefix so
	// the inner patterns (e.g. "GET /api/processes") still match correctly.
	mux.Handle("/api/", srv.Routes())

	// Serve the compiled frontend from web/dist.
	// http.Dir resolves relative to the process working directory at startup.
	fs2 := http.FileServer(http.Dir("web/dist"))
	mux.Handle("/", fs2)

	addr := fmt.Sprintf(":%d", *port)
	fmt.Printf("[rtx] serving on http://localhost%s\n", addr)
	fmt.Printf("[rtx] API available at http://localhost%s/api/\n", addr)
	fmt.Printf("[rtx] frontend served from web/dist\n")

	if err := http.ListenAndServe(addr, mux); err != nil {
		fmt.Fprintf(os.Stderr, "[rtx] server error: %v\n", err)
		return 1
	}
	return 0
}
