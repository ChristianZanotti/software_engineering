# Fahrrad-Verleih – Prototyp (Aufgabe 1 c)

Fahrradverleih statt Ski-Verleih, entwickelt mit **Spec-Driven Development (SDD)**:

- **README.md** (diese Datei) = *Spezifikation* – **was** das System können muss.
- **CLAUDE.md** = *Arbeitsanweisung* – **wie** entwickelt wird.

Das Projekt übernimmt die fachlichen Anforderungen aus dem Ski-Verleih (Aufgabe a + b) und baut sie zu einem
**generischen Verleih-Framework mit Plugin-Architektur** um. Fahrräder sind darin *ein* Plugin.

```
              ┌──────────────┐
              │  RentalItem  │  ← Kern: Item, Person, Ausleihe, Tarif, Dashboard
              │  (Framework) │     kennt keine Fahrräder
              └──────┬───────┘
                     │ Plugin-Interface
          ┌──────────┴──────────┐
   ┌──────┴──────┐       ┌──────┴──────┐
   │ Bike-Plugin │       │ Ski-Plugin… │  ← liefert Typen, Felder, Regeln, Tarife, Zubehör
   │  (aktiv)    │       │ (möglich)   │
   └─────────────┘       └─────────────┘
```

Anforderungs-IDs: `K…` = Kern (aus Aufgabe a/b übernommen), `C…` = neu in Aufgabe c. Tests tragen die ID im Namen.

---

## Schnellstart

```bash
go run .            # http://localhost:8080   (PORT=9000 go run . für anderen Port)
go test ./...
```

Nur Go ≥ 1.22, keine externen Abhängigkeiten. Beispieldaten: 20 Fahrräder, 10 Personen, 3 laufende Ausleihen, 2 Reservierungen.

---

## Glossar

| Begriff | Bedeutung |
|---|---|
| **Item** (RentalItem) | Verleihbarer Gegenstand. Gehört zu einer **Kategorie** (= Plugin, z. B. `bike`) und hat einen **Typ** (z. B. `ebike`) |
| **Plugin** | Liefert für eine Kategorie: Typen mit Starttarifen, Zusatzfelder, Prüfregeln, Zubehör, Anzeigenamen |
| **Attribut** | Plugin-spezifisches Feld eines Items (z. B. Rahmengröße) |
| **Zubehör** | Optional buchbare Extras mit Pauschalpreis pro Ausleihe (z. B. Helm) |
| **Person, Ausleihe, Reservierung, Tarif** | wie im Ski-Verleih |

Status einer Ausleihe: `reserviert` → `aktiv` → (`ueberfaellig`) → `zurueckgegeben`, bzw. `storniert` – abgeleitet aus „jetzt".

---

## Kern-Anforderungen (aus Aufgabe a + b übernommen)

| ID | Anforderung | Akzeptanzkriterien |
|---|---|---|
| **K1** | Dashboard | Kacheln *Verfügbare Fahrräder*, *Ausgeliehene Fahrräder*, *Gesamt*, *Registrierte Personen*; Zusatzkennzahlen (Reservierungen, überfällig, Umsatz heute, ohne Ausweis-Scan); Schnellzugriff; Listen Überfällig / Heute fällig / Nächste Reservierungen |
| **K2** | Item-Verwaltung | Anlegen/Bearbeiten/Löschen, Suche + Filter nach Typ/Status; Löschen bei offener Ausleihe → 409 |
| **K3** | Personen-Verwaltung | Vorname*, Nachname*, E-Mail oder Telefon*; Löschen bei offener Ausleihe → 409 |
| **K4** | Ausweis (Nr. + Typ, hochladbar) | Typ (`personalausweis`, `reisepass`, `fuehrerschein`) + Nummer (5–20 Zeichen) Pflicht; Scan JPEG/PNG/WebP/PDF ≤ 5 MB, Typ per Inhalt geprüft; Typ+Nummer eindeutig (409) |
| **K5** | Ausleihe mit Verleihbeginn + Verleihdauer | Dauer 1 h – 30 Tage; Beginn max. 15 min in der Vergangenheit; keine Überschneidung `[Beginn, Ende)` (409); überfälliges Item erst nach Rückgabe wieder verleihbar; Auswahl zeigt nur freie Items |
| **K6** | Rückgabe & Storno | Rückgabe nur aktiv/überfällig, Storno nur reserviert; Rückgabe-Dialog zeigt Endpreis |
| **K7** | Benutzerfreundlichkeit | Deutsche Oberfläche, Dialoge, Toasts, Bestätigung vor destruktiven Aktionen, responsive |
| **K8** | Stundenweise Preise | `Preis = volleTage × Tagespreis + min(restStd × Stundenpreis, Tagespreis)` |
| **K9** | Abrechnung nach tatsächlicher Dauer | Jede angefangene Stunde zählt, mind. 1 h |
| **K10** | Tarife pflegen | Pro Typ änderbar; Tarif wird bei Buchung eingefroren (Snapshot) |
| **K11** | Flexibler Verleihbeginn + Verleihdauer (stundenweise) | Beginn: Umschalter *Sofort* / *Später* (Datum + volle Stunde); Dauer als **Tage + Stunden**; Schnellwahl 1/2/3 Std., Halbtag (4 Std.), 1/2/3 Tage, 1 Woche, **„bis Ladenschluss (19:00)"** (auf volle Stunden aufgerundet, nach Ladenschluss → nächster Tag); Anzeige „Ab … → Rückgabe bis …"; Eingaben wie 26 Std. werden zu 1 Tag 2 Std. normalisiert; Dauer 0 oder > 30 Tage → Hinweis; Preisrechner nutzt dieselbe Eingabe |

---

## Neue Anforderungen Aufgabe c) – Fahrradverleih

### C1 Fahrräder als Verleihgegenstand
- Typen und Starttarife:

| Typ | Key | Stundenpreis | Tagespreis |
|---|---|---|---|
| Citybike | `city` | 3,00 € | 15,00 € |
| Trekkingrad | `trekking` | 4,00 € | 18,00 € |
| Mountainbike | `mtb` | 6,00 € | 28,00 € |
| Rennrad | `rennrad` | 7,00 € | 32,00 € |
| E-Bike | `ebike` | 8,00 € | 39,00 € |
| Lastenrad | `lastenrad` | 9,00 € | 45,00 € |
| Kinderrad | `kinder` | 2,00 € | 9,00 € |

- Felder: Marke*, Modell*, Zustand* (`neu`, `gut`, `gebraucht`), Notiz +
  Plugin-Attribute **Rahmengröße*** (`XS`–`XL`), **Laufradgröße*** (12"–29"), **Gänge** (1–30),
  **Akku-Reichweite km** (20–200, *nur und Pflicht bei E-Bike/Lastenrad*), **Farbe**.

### C2 Plugin-Architektur (Verleih-Framework)
- Der Kern (`internal/domain`, `internal/store`, `internal/handler`, `web/`) enthält **kein** fahrradspezifisches Wissen.
- Ein Plugin implementiert das Interface `domain.Plugin` und liefert: Kategorie-Key, Anzeigenamen (Singular/Plural),
  Typen mit Starttarifen, Felddefinitionen, Zubehör, Prüfregeln, Anzeige-Label.
- Generische Feldprüfung im Kern: Pflichtfeld, Zahlenbereich, Auswahlwerte, „nur bei Typ X".
- Die Oberfläche erzeugt Formulare, Tabellenspalten, Filter und Zubehör-Auswahl **aus den Plugin-Metadaten** (`GET /api/plugins`).
- Akzeptanz: Ein neuer Artikeltyp (z. B. Ski, Teil d) braucht **nur** eine neue Plugin-Datei + eine Zeile Registrierung in `main.go`.
  Das wird durch einen Test mit einem Test-Plugin nachgewiesen.

### C3 Zubehör
- Pro Ausleihe optional buchbar, Pauschalpreis je Ausleihe:
  Helm 2,00 €, Schloss 1,00 €, Korb 1,50 €, Packtaschen 3,00 €, Kindersitz 4,00 €, Ersatz-Akku 6,00 € (*nur E-Bike/Lastenrad*).
- Akzeptanz: Zubehör, das für den Typ nicht erlaubt ist, wird abgelehnt (400). Zubehör-Preise werden bei der Buchung eingefroren
  und kommen auf den geplanten **und** den finalen Preis.

### C4 Fahrradspezifische Regeln
- Kinderrad: Laufrad höchstens 24", Rahmengröße `XS` oder `S`.
- E-Bike/Lastenrad: Akku-Reichweite ist Pflicht.

### C5 Verfügbarkeit nach Typ
- Dashboard zeigt pro Fahrradtyp „verfügbar / gesamt" (z. B. *E-Bike 3 / 4*).

### C6 Preisvorschau mit Zubehör
- Live-Preis beim Anlegen enthält Mietpreis + Zubehör mit Rechenweg; Preisrechner auf Seite „Preise".

---

## REST-API

JSON; Fehler `{"error": "…"}` mit 400/404/409. Zeiten RFC 3339, Geld in Cent.

| Methode | Pfad | Zweck | Anf. |
|---|---|---|---|
| GET | `/api/plugins` | App-Name + Metadaten aller Plugins (Typen, Felder, Zubehör) | C2 |
| GET | `/api/dashboard` | Kennzahlen, Listen, Verfügbarkeit nach Typ | K1, C5 |
| GET, POST | `/api/items` | Items auflisten / anlegen | K2, C1 |
| GET, PUT, DELETE | `/api/items/{id}` | Item lesen / ändern / löschen | K2 |
| GET | `/api/items/available?start=…&hours=…` | freie Items im Zeitraum | K5 |
| GET, POST | `/api/persons` | Personen | K3 |
| GET, PUT, DELETE | `/api/persons/{id}` | Person | K3 |
| GET, POST, DELETE | `/api/persons/{id}/document` | Ausweis-Scan (multipart `file`) | K4 |
| GET, POST | `/api/rentals` | Ausleihen (`{itemId, personId, start, durationHours, extras: ["helm"]}`) | K5, C3 |
| GET | `/api/rentals/quote?itemId=…\|category=…&kind=…&hours=…&extras=helm,schloss` | Preisvorschau | C6 |
| GET | `/api/rentals/{id}/return-quote` | Endpreis-Vorschau | K6, K9 |
| POST | `/api/rentals/{id}/return` | Rückgabe | K6, K9 |
| POST | `/api/rentals/{id}/cancel` | Storno | K6 |
| GET | `/api/prices` | Tarife | K10 |
| PUT | `/api/prices/{category}/{kind}` | Tarif ändern | K10 |

---

## Nicht-Ziele
- Keine Persistenz, keine Anmeldung, keine Zahlungsabwicklung.
- Plugins werden zur Compile-Zeit registriert (kein dynamisches Nachladen von `.so`-Dateien – bewusst einfach gehalten).

## Umsetzungsstatus
- [x] K1–K11 Kern aus Aufgabe a + b (inkl. flexibler Dauer-/Beginn-Eingabe)
- [x] C1 Fahrräder als Verleihgegenstand
- [x] C2 Plugin-Architektur inkl. Nachweis-Test
- [x] C3 Zubehör
- [x] C4 Fahrradspezifische Regeln
- [x] C5 Verfügbarkeit nach Typ
- [x] C6 Preisvorschau mit Zubehör
