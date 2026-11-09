# CLAUDE.md – Ski-Verleih (Prototyp)

Nur Prototyp. Priorität: schnell verständlich, leicht änderbar.
**Die Spezifikation steht in `README.md`. Sie ist die einzige Quelle der Wahrheit für fachliche Anforderungen.**

## SDD-Workflow (immer einhalten)
1. Vor jeder Änderung `README.md` lesen und die betroffene Anforderungs-ID nennen (`A1`–`A8`, `B1`–`B6`).
2. Neue oder geänderte Anforderung **zuerst** in `README.md` eintragen (inkl. Akzeptanzkriterien), **dann** implementieren.
3. Business-Logik mit Tests absichern, die die Akzeptanzkriterien abbilden. Testname enthält die ID (`TestA5_…`, `TestB2_…`).
4. Vor Abschluss: `gofmt -w .`, `go vet ./...`, `go test ./...` müssen sauber sein.
5. Abschnitt „Umsetzungsstatus" in `README.md` aktualisieren.
6. Weicht der Code von der Spezifikation ab, gilt die Spezifikation. Bei Widersprüchen nachfragen statt raten.

## Stack
- Backend: Go, nur Standardbibliothek („objektorientiert": Structs + Methoden, Interfaces nur wo sinnvoll)
- DB: In-Memory (Maps + `sync.RWMutex`), kein SQL, kein ORM
- Frontend: reines HTML/CSS/JS (kein Build-Schritt, kein Framework), per `embed` ausgeliefert
- Versionierung: lokales Git, Abgabe als ZIP bzw. öffentliches GitHub-Repo

## Commands
- Starten: `go run .` → http://localhost:8080 (`PORT` überschreibt den Port)
- Tests: `go test ./...`
- Format: `gofmt -w .`
- Statische Prüfung: `go vet ./...`

## Architektur
```
main.go                 Einstieg: Store anlegen, Seed laden, Routing, Static-Files (embed)
internal/domain/        Kern-Structs + Methoden (Ski, Person, Rental, Tariff, Dashboard) – keine I/O
internal/store/         MemoryStore (thread-safe) + Seed-Daten; prüft Konflikte atomar unter Lock
internal/handler/       HTTP-Handler: JSON rein/raus, Fehler → Statuscode, View-Structs fürs Frontend
web/                    index.html, style.css, app.js (Single Page, Hash-Routing)
```
- Domain kennt weder HTTP-Handler noch Store. Store kennt Domain. Handler kennt beide.
- Zeitabhängige Logik bekommt `now time.Time` als Parameter (testbar, keine versteckten `time.Now()` in der Domain).
- Keine globalen Variablen außer dem eingebetteten `web`-Dateisystem.
- Fehler mit `fmt.Errorf("…: %w", err)` wrappen, nicht verschlucken.
- Fehlerarten: `domain.ErrValidation` → 400, `domain.ErrNotFound` → 404, `domain.ErrConflict` → 409.

## Konventionen
- Code-Bezeichner Englisch, Oberfläche und Fehlermeldungen Deutsch.
- JSON-Felder camelCase; Enum-Werte als ASCII-Kleinbuchstaben (`ueberfaellig`, `fuehrerschein`).
- Geld immer als `int64` in Cent (`…Cents`), nie `float`.
- Zeiten als `time.Time`, in der API RFC 3339.
- Methoden an Structs hängen statt freier Funktionen, wo es Sinn ergibt.
- Kein Dependency-Injection-Framework, keine neuen Dependencies ohne Nachfrage.
- Store gibt Kopien zurück, nie Pointer in seine Maps.
- Keine Secrets im Code.

## Frontend
- Möglichst schlankes Vanilla-JS, alle Daten nur über `/api/...` (nie Logik doppelt im Frontend – Preise kommen vom Server).
- Deutsche Formatierung: `Intl.NumberFormat('de-DE')`, `toLocaleString('de-DE')`.
- Benutzerfreundlich: Dialoge statt Seitenwechsel, Toasts für Feedback, Bestätigung vor destruktiven Aktionen.
- Farbschema wie Beispiel-Dashboard: dunkle Navigationsleiste, Kacheln mit Farbverlauf.

## Regeln
- Keine generierten Dateien von Hand editieren
- Commit-Messages: kurz und imperativ, mit Anforderungs-ID (`A5: Doppelbuchung verhindern`)
- Bei Unklarheit fragen statt raten
- Prototyp-Fokus: lieber einfach und klar als elegant und komplex
