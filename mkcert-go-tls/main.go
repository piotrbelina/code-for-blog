package main

import (
	"io"
	"log"
	"net/http"
)

func main() {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, req *http.Request) {
		_, err := io.WriteString(w, "Hello World, TLS!\n")
		if err != nil {
			log.Fatal(err)
		}
	})
	log.Println("Starting listening on port 8443, open https://127.0.0.1:8443")
	log.Fatal(http.ListenAndServeTLS(":8443", "example.com+5.pem", "example.com+5-key.pem", mux))
}
