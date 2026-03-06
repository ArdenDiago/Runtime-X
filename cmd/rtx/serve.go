package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"runtimex/internal/api"
	"runtimex/internal/scheduler"
)

// gracefulShutdownTimeout is the maximum time allowed for the HTTP server to
// drain in-flight requests and for the scheduler to stop all managed processes.
const gracefulShutdownTimeout = 10 * time.Second

// cmdServe implements `rtx serve [-port <n>]`.
// It starts the REST API server backed by a new Scheduler, serves the React
// frontend from web/dist, and shuts down cleanly on SIGINT or SIGTERM:
//  1. httpServer.Shutdown(ctx) stops accepting new requests and drains existing ones.
//  2. scheduler.StopAll() terminates all managed processes in parallel.
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
	// srv.Routes() returns a CORS-wrapped handler; the patterns inside already
	// include the /api prefix, so no stripping is required.
	mux.Handle("/api/", srv.Routes())

	// Serve the compiled frontend from web/dist.
	// http.Dir resolves relative to the process working directory at startup.
	fsHandler := http.FileServer(http.Dir("web/dist"))
	mux.Handle("/", fsHandler)

	addr := fmt.Sprintf(":%d", *port)
	httpServer := &http.Server{
		Addr:    addr,
		Handler: mux,
	}

	// Run the HTTP server in a goroutine so the main goroutine can listen for
	// termination signals without blocking.
	serverErrCh := make(chan error, 1)
	go func() {
		fmt.Printf("[rtx] serving on http://localhost%s\n", addr)
		fmt.Printf("[rtx] API available at http://localhost%s/api/\n", addr)
		fmt.Printf("[rtx] frontend served from web/dist\n")

		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			serverErrCh <- err
		}
		close(serverErrCh)
	}()

	// Block until SIGINT or SIGTERM is received, or the server fails to start.
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	select {
	case sig := <-quit:
		fmt.Printf("\n[rtx] received %s — initiating graceful shutdown\n", sig)
	case err := <-serverErrCh:
		fmt.Fprintf(os.Stderr, "[rtx] server error: %v\n", err)
		return 1
	}

	// Graceful shutdown: allow up to gracefulShutdownTimeout for in-flight
	// requests to complete and for all managed processes to be terminated.
	ctx, cancel := context.WithTimeout(context.Background(), gracefulShutdownTimeout)
	defer cancel()

	// Stop accepting new HTTP requests and drain existing ones.
	if err := httpServer.Shutdown(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "[rtx] HTTP shutdown error: %v\n", err)
	}

	// Terminate all managed processes in parallel.
	fmt.Printf("[rtx] stopping all managed processes...\n")
	sched.StopAll()

	fmt.Printf("[rtx] shutdown complete\n")
	return 0
}
