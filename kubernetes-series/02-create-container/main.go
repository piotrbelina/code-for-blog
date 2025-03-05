package main

import (
	"fmt"
	"log/slog"
	"net/http"
	"os"

	"github.com/piotrbelina/code-for-blog/kubernetes-series/02-create-container/api"
)

func main() {
	logHandler := slog.NewJSONHandler(os.Stdout, nil)
	log := slog.New(logHandler)

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "Hello World")
	})

	handler := api.HandlerFromMux(api.NewStrictHandler(api.NewTodoStore(), nil), mux)

	addr := ":8000"
	server := &http.Server{
		ErrorLog: slog.NewLogLogger(logHandler, slog.LevelError),
		Handler:  handler,
		Addr:     addr,
	}

	log.Info("Starting listening", slog.String("addr", addr))
	err := server.ListenAndServe()
	if err != nil {
		log.Error("Error starting server", slog.String("err", err.Error()))
		os.Exit(1)
	}
}
