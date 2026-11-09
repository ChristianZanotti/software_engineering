# Ski-Verleih – Prototyp (Aufgabe 1 a + b)

Verleihsystem für Ski, entwickelt mit **Spec-Driven Development (SDD)**:

- **README.md** (diese Datei) = *Spezifikation* – **was** das System können muss (Anforderungen mit IDs + Akzeptanzkriterien).
- **CLAUDE.md** = *Arbeitsanweisung* – **wie** entwickelt wird (Stack, Architektur, Konventionen, Workflow).

Jede Funktion im Code lässt sich auf eine Anforderungs-ID (`A…` für Aufgabe a, `B…` für Aufgabe b) zurückführen;
Tests tragen die ID im Namen (z. B. `TestB2_PriceCalculation`).

---

## Schnellstart

```bash
go run .            # startet Server auf http://localhost:8080  (Port änderbar: PORT=9000 go run .)
go test ./...       # alle Tests
```

Keine externen Abhängigkeiten – nur Go ≥ 1.22 (Standardbibliothek). Das Frontend ist per `embed` im Binary enthalten.
Beim Start werden Beispieldaten geladen (20 Ski, 10 Personen, 3 laufende Ausleihen, 2 Reservierungen).

---

## Glossar

| Begriff | Bedeutung |
|---|---|
| **Ski** | Verleihbares Paar Ski bzw. Snowboard mit Typ, Marke, Modell, Länge, Zustand |
| **Person** | Kunde; muss einen Ausweis (Typ + Nummer) angeben, Ausweis-Scan kann hochgeladen werden |
| **Ausleihe** | Verknüpfung Ski ↔ Person mit **Verleihbeginn** und **Verleihdauer** |
| **Reservierung** | Ausleihe, deren Beginn in der Zukunft liegt |
| **Tarif** | Preis pro Ski-Typ: Stundenpreis + Tagespreis (Aufgabe b) |

### Status einer Ausleihe (abgeleitet aus Zeitpunkt „jetzt")

| Status | Bedingung |
|---|---|
| `reserviert` | jetzt < Beginn |
| `aktiv` | Beginn ≤ jetzt < geplantes Ende |
| `ueberfaellig` | jetzt ≥ geplantes Ende und noch nicht zurückgegeben |
| `zurueckgegeben` | Rückgabe wurde gebucht |
| `storniert` | Reservierung wurde vor Beginn storniert |

---

## Spezifikation Aufgabe a) – Ski-Verleih mit Dashboard

### A1 Dashboard
- Startseite zeigt vier Kennzahl-Kacheln: **Verfügbare Ski**, **Ausgeliehene Ski**, **Gesamt Ski**, **Registrierte Personen**.
- Zusatz-Kennzahlen: offene Reservierungen, überfällige Ausleihen, Umsatz heute, Personen ohne Ausweis-Scan.
- **Schnellzugriff**: Buttons „Ski verwalten", „Personen verwalten", „Neue Ausleihe".
- Listen: **Überfällig** (mit Rückgabe-Button), **Heute fällig**, **Nächste Reservierungen**.
- Akzeptanz: *Ausgeliehen* = Ski mit Ausleihe im Status `aktiv` oder `ueberfaellig`; *Verfügbar* = Gesamt − Ausgeliehen.

### A2 Ski-Verwaltung
- Ski anlegen, bearbeiten, löschen. Felder: Marke*, Modell*, Typ* (`alpin`, `carving`, `freeride`, `langlauf`, `touren`, `snowboard`), Länge in cm* (80–220), Zustand* (`neu`, `gut`, `gebraucht`), Notiz.
- Tabelle zeigt aktuellen Status (verfügbar / ausgeliehen / überfällig) und nächste Reservierung.
- Suche (Marke/Modell) und Filter nach Typ und Status.
- Akzeptanz: Ein Ski mit offener Ausleihe (reserviert/aktiv/überfällig) kann **nicht gelöscht** werden (HTTP 409).

### A3 Personen-Verwaltung
- Person anlegen, bearbeiten, löschen. Felder: Vorname*, Nachname*, E-Mail, Telefon (mind. eines von beiden*).
- Akzeptanz: Person mit offener Ausleihe kann nicht gelöscht werden (409).

### A4 Ausweis (Nr. + Typ, hochladbar)
- Pflichtfelder bei jeder Person: **Ausweistyp** (`personalausweis`, `reisepass`, `fuehrerschein`) und **Ausweisnummer** (5–20 Zeichen, Buchstaben/Ziffern).
- Ausweis-Scan **hochladbar** (JPEG, PNG, WebP oder PDF, max. 5 MB), per Klick oder Drag & Drop; ansehen und entfernen möglich.
- Akzeptanz: Dateityp wird anhand des Inhalts geprüft (nicht nur Dateiendung). Gleiche Kombination Typ + Nummer darf nur einmal existieren (409).

### A5 Ausleihe anlegen (Verleihbeginn + Verleihdauer)
- Eingaben: Person, **Verleihbeginn**, **Verleihdauer** (Eingabeform siehe B1), Ski.
- Die Ski-Auswahl zeigt nur Ski, die im gewählten Zeitraum frei sind.
- Akzeptanz:
  - Dauer 1 Stunde bis 30 Tage (720 h).
  - Beginn darf höchstens 15 Minuten in der Vergangenheit liegen.
  - Ein Ski darf nicht doppelt belegt werden: Zeiträume `[Beginn, Ende)` offener Ausleihen dürfen sich nicht überschneiden (409).
  - Ein **überfälliger** Ski kann erst nach seiner Rückgabe neu verliehen werden.

### A6 Rückgabe & Stornierung
- Rückgabe für `aktiv`/`ueberfaellig` → Status `zurueckgegeben`, Rückgabezeit wird gespeichert, Endpreis berechnet (siehe B5).
- Stornierung nur für `reserviert`.
- Ausleihen-Liste mit Filter (Offen / Reserviert / Aktiv / Überfällig / Abgeschlossen / Alle) und Suche.

### A7 Benutzerfreundlichkeit
- Komplett deutsche Oberfläche, verständliche Fehlermeldungen direkt aus der API.
- Formulare in Dialogen, Pflichtfelder markiert, Erfolgs-/Fehlermeldungen als Toast.
- Sicherheitsabfrage vor Löschen/Stornieren/Rückgabe; Rückgabe-Dialog zeigt den Endpreis (vom Server berechnet, mit Tarif-Snapshot).
- Responsive Layout (nutzbar auf Tablet am Verleihtresen).

### A8 Beispieldaten
- Beim Start: 20 Ski, 10 Personen (6 davon mit Muster-Ausweis-Scan), 3 laufende Ausleihen, 2 Reservierungen, 1 abgeschlossene Ausleihe.

---

## Spezifikation Aufgabe b) – Flexibilisierung von Verleihdauer, Verleihbeginn und Preisen (stundenweise)

Tafel: *„b) Flexibilisierung der Verleihdauer und des Verleihbeginns → stundenweise; • Preise"*

### B1 Flexibler Verleihbeginn und flexible Verleihdauer (stundenweise)
- **Verleihbeginn**: Umschalter *Sofort* (= jetzt, Laufkundschaft) oder *Später* (Reservierung) mit **Datum + volle Stunde**.
- **Verleihdauer** stundengenau als **Tage + Stunden** (z. B. 1 Tag 3 Std.), statt nur ganzer Tage.
- **Schnellwahl** für typische Dauern: 1 Std., 2 Std., 3 Std., Halbtag (4 Std.), 1 Tag, 2 Tage, 3 Tage, 1 Woche
  sowie **„bis Liftschluss (16:30)"** (Dauer wird auf volle Stunden aufgerundet).
- Das berechnete **Rückgabedatum** und der Preis werden bei jeder Änderung sofort angezeigt.
- Akzeptanz: Dauer 1 h – 30 Tage (720 h), siehe A5; Stunden-Feld 0–23, bei 23 Std. + 1 springt die Anzeige auf den nächsten Tag;
  Dauer 0 → Hinweis, Buchen nicht möglich. Preisrechner (B4) nutzt dieselbe Eingabe.

### B2 Tarif pro Ski-Typ
- Jeder Ski-Typ hat einen **Stundenpreis** und einen **Tagespreis** (Tagesdeckel).
- Akzeptanz: Beide > 0, Tagespreis ≥ Stundenpreis.

| Typ | Stundenpreis | Tagespreis |
|---|---|---|
| alpin | 8,00 € | 35,00 € |
| carving | 9,00 € | 39,00 € |
| freeride | 11,00 € | 49,00 € |
| langlauf | 5,00 € | 22,00 € |
| touren | 10,00 € | 45,00 € |
| snowboard | 8,00 € | 35,00 € |

### B3 Preisberechnung
Abgerechnet wird **stundenweise**, ein Tag kostet höchstens den Tagespreis:

```
volleTage  = Stunden / 24      (ganzzahlig)
restStd    = Stunden % 24
Preis      = volleTage × Tagespreis + min(restStd × Stundenpreis, Tagespreis)
```

Akzeptanzbeispiele (Tarif alpin 8 €/h, 35 €/Tag):

| Dauer | Preis | Rechnung |
|---|---|---|
| 1 h | 8,00 € | 1 × 8 |
| 3 h | 24,00 € | 3 × 8 |
| 5 h | 35,00 € | 40 → gedeckelt auf 35 |
| 24 h | 35,00 € | 1 Tag |
| 26 h | 51,00 € | 35 + 2 × 8 |
| 30 h | 70,00 € | 35 + min(48, 35) |

### B4 Preisvorschau
- Beim Anlegen einer Ausleihe wird der Preis live mit Rechenweg angezeigt (`GET /api/rentals/quote`).
- Seite „Preise" enthält einen Preisrechner (Typ + Dauer).

### B5 Abrechnung nach tatsächlicher Dauer
- Bei Rückgabe wird nach **tatsächlicher Nutzungsdauer** abgerechnet; jede angefangene Stunde zählt (mind. 1 h).
- Früher zurück → günstiger, später zurück → teurer. Die Tabelle zeigt geplanten und finalen Preis.

### B6 Tarife pflegen
- Tarife sind in der Oberfläche (Seite „Preise") änderbar.
- Akzeptanz: Der Tarif wird beim Anlegen einer Ausleihe **eingefroren** (Snapshot). Spätere Tarifänderungen wirken nur auf neue Ausleihen.

---

## REST-API

Alle Antworten JSON; Fehler als `{"error": "…"}` mit 400 (Validierung), 404 (nicht gefunden), 409 (Konflikt).
Zeiten im Format RFC 3339, Geldbeträge in **Cent** (`…Cents`).

| Methode | Pfad | Zweck | Anf. |
|---|---|---|---|
| GET | `/api/dashboard` | Kennzahlen + Listen | A1 |
| GET, POST | `/api/skis` | Ski auflisten / anlegen | A2 |
| GET, PUT, DELETE | `/api/skis/{id}` | Ski lesen / ändern / löschen | A2 |
| GET | `/api/skis/available?start=…&hours=…` | freie Ski im Zeitraum | A5 |
| GET, POST | `/api/persons` | Personen auflisten / anlegen | A3 |
| GET, PUT, DELETE | `/api/persons/{id}` | Person lesen / ändern / löschen | A3 |
| GET, POST, DELETE | `/api/persons/{id}/document` | Ausweis-Scan laden / hochladen (multipart `file`) / löschen | A4 |
| GET, POST | `/api/rentals` | Ausleihen auflisten / anlegen | A5 |
| GET | `/api/rentals/{id}/return-quote` | Endpreis-Vorschau, falls jetzt zurückgegeben wird | A7, B5 |
| POST | `/api/rentals/{id}/return` | Rückgabe buchen | A6, B5 |
| POST | `/api/rentals/{id}/cancel` | Reservierung stornieren | A6 |
| GET | `/api/rentals/quote?skiId=…\|type=…&hours=…` | Preisvorschau | B4 |
| GET | `/api/prices` | Tarife | B2 |
| PUT | `/api/prices/{type}` | Tarif ändern | B6 |

Beispiel Ausleihe anlegen:

```bash
curl -X POST localhost:8080/api/rentals -H 'Content-Type: application/json' \
  -d '{"skiId":4,"personId":2,"start":"2026-12-01T09:00:00+01:00","durationHours":5}'
```

---

## Nicht-Ziele (Prototyp)
- Keine Datenbank – alle Daten liegen im Speicher und sind nach Neustart weg.
- Keine Benutzer-Anmeldung, keine Zahlungsabwicklung, keine Rechnungs-PDFs.

## Umsetzungsstatus

- [x] A1 Dashboard
- [x] A2 Ski-Verwaltung
- [x] A3 Personen-Verwaltung
- [x] A4 Ausweis (Nr. + Typ, Upload)
- [x] A5 Ausleihe mit Beginn + Dauer, Doppelbuchungs-Schutz
- [x] A6 Rückgabe & Stornierung
- [x] A7 Benutzerfreundlichkeit
- [x] A8 Beispieldaten
- [x] B1 Flexibler Verleihbeginn + Verleihdauer (stundenweise, Schnellwahl)
- [x] B2 Tarif pro Ski-Typ
- [x] B3 Stundenweise Preisberechnung mit Tagesdeckel
- [x] B4 Preisvorschau + Preisrechner
- [x] B5 Abrechnung nach tatsächlicher Dauer
- [x] B6 Tarife pflegen, Snapshot bei Buchung
