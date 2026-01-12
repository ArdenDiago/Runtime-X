package main

import (
	"fmt"
	"log"
	"net/http"
)

func main() {
	log.Println("API Service starting on :8080")

	http.HandleFunc("/health", healthHandler)

	err := http.ListenAndServe(":8080", nil)
	if err != nil {
		log.Fatalf("server failed: %v", err)
	}
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	fmt.Fprintln(w, "OK")
}
