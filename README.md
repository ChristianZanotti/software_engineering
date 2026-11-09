# Software Engineering – Verleih-Prototypen

Zwei Go-Prototypen, entwickelt mit **Spec-Driven Development** (README = Spezifikation, CLAUDE.md = Arbeitsanweisung).

| Projekt | Inhalt |
|---|---|
| [`ski-verleih/`](ski-verleih/) | Aufgabe 1 a + b: Ski-Verleihsystem |
| [`fahrrad-verleih/`](fahrrad-verleih/) | Aufgabe 1 c: generisches Verleih-Framework mit Plugin-Architektur (Fahrrad-Plugin) |

`Template CLAUDE.rtf` ist die gemeinsame CLAUDE.md-Vorlage für beide Projekte.

## Schnellstart

Voraussetzung: Go ≥ 1.22, keine externen Abhängigkeiten.

```bash
cd ski-verleih        # oder: cd fahrrad-verleih
go run .              # http://localhost:8080
go test ./...
```
