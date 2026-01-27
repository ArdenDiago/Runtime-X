package main

import (
	"log"
	"net/http"

	"runtimex/cmd/api/docker"
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

	// 👇 Mount all /docker routes here
	mux.Handle("/docker/", docker.Router())

	log.Println("API started on :8080")
	log.Fatal(http.ListenAndServe(":8080", mux))
}
