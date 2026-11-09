// Fahrrad-Verleih – Prototyp für Aufgabe 1 c auf Basis eines Verleih-Frameworks mit Plugins.
// Spezifikation: README.md, Arbeitsregeln: CLAUDE.md.
package main

import (
	"embed"
	"io/fs"
	"log"
	"net/http"
	"os"
	"time"

	"fahrradverleih/internal/domain"
	"fahrradverleih/internal/handler"
	"fahrradverleih/internal/plugins/bike"
	"fahrradverleih/internal/store"
)

//go:embed web
var webFiles embed.FS

func main() {
	// C2: Plugins registrieren. Ein weiterer Artikeltyp = eine weitere Zeile, z. B. ski.Plugin{}.
	reg := domain.NewRegistry(
		bike.Plugin{},
	)

	st := store.NewMemoryStore(reg)
	store.Seed(st, bike.SampleItems(), time.Now())

	static, err := fs.Sub(webFiles, "web")
	if err != nil {
		log.Fatalf("web-Dateien: %v", err)
	}

	mux := http.NewServeMux()
	handler.New("Fahrrad-Verleih", st).Register(mux)
	mux.Handle("/", http.FileServerFS(static))

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	log.Printf("Fahrrad-Verleih läuft auf http://localhost:%s", port)
	log.Fatal(http.ListenAndServe(":"+port, mux))
}
