# CLAUDE.md – Fahrrad-Verleih (Prototyp, Verleih-Framework)

Nur Prototyp. Priorität: schnell verständlich, leicht änderbar.
**Die Spezifikation steht in `README.md`** (Kern `K1`–`K11`, neu `C1`–`C6`). Sie ist die einzige Quelle der Wahrheit.

## SDD-Workflow (immer einhalten)
1. Vor jeder Änderung `README.md` lesen und die betroffene Anforderungs-ID nennen.
2. Neue/geänderte Anforderung **zuerst** in `README.md` (mit Akzeptanzkriterien), **dann** implementieren.
3. Tests bilden Akzeptanzkriterien ab, Testname enthält die ID (`TestC3_…`, `TestK5_…`).
4. Vor Abschluss: `gofmt -w .`, `go vet ./...`, `go test ./...` sauber.
5. „Umsetzungsstatus" in `README.md` aktualisieren.
6. Bei Widerspruch zwischen Code und Spezifikation gilt die Spezifikation; bei Unklarheit fragen.

## Stack
- Backend: Go, nur Standardbibliothek (Structs + Methoden; Interfaces nur wo nötig – hier: `domain.Plugin`)
- DB: In-Memory (Maps + `sync.RWMutex`)
- Frontend: reines HTML/CSS/JS, per `embed` ausgeliefert, kein Build-Schritt

## Commands
- Starten: `go run .` → http://localhost:8080 (`PORT` überschreibt)
- Tests: `go test ./...` · Format: `gofmt -w .` · Prüfung: `go vet ./...`

## Architektur
```
main.go                    Einstieg: Plugins registrieren, Store, Seed, Routing, Static-Files
internal/domain/           Framework-Kern: Item, Person, Rental, Tariff, Dashboard,
                           Plugin-Interface + Registry (generische Feldprüfung)
internal/plugins/bike/     Fahrrad-Plugin: Typen, Felder, Zubehör, Regeln, Seed-Daten
internal/store/            MemoryStore (thread-safe), prüft Konflikte atomar unter Lock
internal/handler/          HTTP ⇄ Store, View-Structs, Fehler → Statuscode
web/                       Generisches SPA, rendert alles aus /api/plugins
```
- **Plugin-Regel (C2):** Kein Fahrrad-Wissen außerhalb von `internal/plugins/bike/`.
  Kein `if category == "bike"` im Kern oder im Frontend. Texte wie „Fahrrad" kommen aus den Plugin-Metadaten.
- Neues Plugin = neues Paket unter `internal/plugins/<name>/`, das `domain.Plugin` implementiert, + Registrierung in `main.go`.
- Domain kennt weder Store noch HTTP. Zeitabhängige Logik bekommt `now time.Time` als Parameter.
- Fehler: `domain.ErrValidation` → 400, `ErrNotFound` → 404, `ErrConflict` → 409; mit `%w` wrappen.

## Konventionen
- Code-Bezeichner Englisch, UI und Fehlermeldungen Deutsch.
- JSON camelCase, Enum-Werte ASCII-Kleinbuchstaben.
- Geld `int64` in Cent, nie `float`. Zeiten `time.Time`, API RFC 3339.
- Tarif-Schlüssel: `"<category>/<kind>"`, z. B. `bike/ebike`.
- Preise (Tarif + Zubehör) werden bei Buchung in der Ausleihe eingefroren.
- Store gibt Kopien zurück. Keine neuen Dependencies ohne Nachfrage. Keine Secrets im Code.

## Frontend
- Vanilla-JS, nur über `/api/...`; Preise immer vom Server (keine Preislogik im Frontend).
- Formulare/Spalten/Filter werden aus Plugin-Feldern generiert (`text`, `number`, `select`, `onlyKinds`).
- Gleiche UX-Regeln wie Ski-Verleih: Dialoge, Toasts, Bestätigungen, deutsche Formatierung, responsive.

## Regeln
- Keine generierten Dateien von Hand editieren
- Commit-Messages kurz, imperativ, mit ID (`C3: Zubehör buchbar`)
- Bei Unklarheit fragen statt raten
- Prototyp-Fokus: lieber einfach und klar als elegant und komplex
