package main

import (
	"html/template"
	"log"
	"net/http"
)

func main() {
	tmpl := template.Must(template.ParseFiles("templates/index.html"))

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		tmpl.Execute(w, nil)
	})

	log.Println("Frontend running on :3000")
	log.Fatal(http.ListenAndServe(":3000", nil))
}
