package main

import (
	"log"
	"net/http"

	dockerRoutes "runtimex/cmd/api/docker"
)

func main() {
	mux := http.NewServeMux()

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Welcome to our home page"))
	})

	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	// ✅ NOTE THE TRAILING SLASH
	mux.Handle("/docker/", dockerRoutes.Router())

	log.Println("API started on :8080")
	log.Fatal(http.ListenAndServe(":8080", mux))
}
