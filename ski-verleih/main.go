// Ski-Verleih – Prototyp für Aufgabe 1 a + b. Spezifikation: README.md, Arbeitsregeln: CLAUDE.md.
package main

import (
	"embed"
	"io/fs"
	"log"
	"net/http"
	"os"
	"time"

	"skiverleih/internal/handler"
	"skiverleih/internal/store"
)

//go:embed web
var webFiles embed.FS

func main() {
	st := store.NewMemoryStore()
	store.Seed(st, time.Now())

	static, err := fs.Sub(webFiles, "web")
	if err != nil {
		log.Fatalf("web-Dateien: %v", err)
	}

	mux := http.NewServeMux()
	handler.New(st).Register(mux)
	mux.Handle("/", http.FileServerFS(static))

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	log.Printf("Ski-Verleih läuft auf http://localhost:%s", port)
	log.Fatal(http.ListenAndServe(":"+port, mux))
}
