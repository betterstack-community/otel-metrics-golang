package main

import (
	"log"
	"net/http"
)

func main() {
	mux := http.NewServeMux()

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("Hello world!"))
	})

	log.Println("Starting HTTP server on port 8000")

	if err := http.ListenAndServe(":8000", mux); err != nil {
		log.Fatal("Server failed to start:", err)
	}
}
