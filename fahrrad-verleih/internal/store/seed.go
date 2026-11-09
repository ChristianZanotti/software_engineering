package store

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"log"
	"time"

	"fahrradverleih/internal/domain"
)

// Seed lädt die Beispieldaten (Items kommen vom Plugin). Es werden nur öffentliche Store-Methoden benutzt,
// d. h. alle Daten durchlaufen dieselben Prüfungen wie echte Eingaben.
func Seed(s *MemoryStore, items []domain.Item, now time.Time) {
	for _, it := range items {
		must(s.CreateItem(it))
	}

	persons := []domain.Person{
		{FirstName: "Anna", LastName: "Huber", Email: "anna.huber@example.com", Phone: "+49 171 1234567", IDDocType: domain.DocIDCard, IDDocNumber: "L01X00T47"},
		{FirstName: "Lukas", LastName: "Maier", Email: "lukas.maier@example.com", IDDocType: domain.DocPassport, IDDocNumber: "C01X00T47"},
		{FirstName: "Sophie", LastName: "Schneider", Phone: "+49 160 9876543", IDDocType: domain.DocIDCard, IDDocNumber: "T22000129"},
		{FirstName: "Jonas", LastName: "Fischer", Email: "jonas.f@example.com", IDDocType: domain.DocLicense, IDDocNumber: "B072RRE2I55"},
		{FirstName: "Mia", LastName: "Weber", Email: "mia.weber@example.com", IDDocType: domain.DocIDCard, IDDocNumber: "LZ6311T47"},
		{FirstName: "Felix", LastName: "Wagner", Phone: "+43 664 1122334", IDDocType: domain.DocPassport, IDDocNumber: "P4366918"},
		{FirstName: "Lena", LastName: "Becker", Email: "lena.becker@example.com", IDDocType: domain.DocIDCard, IDDocNumber: "L7H0R1T29"},
		{FirstName: "Paul", LastName: "Hofmann", Email: "paul.h@example.com", IDDocType: domain.DocLicense, IDDocNumber: "J010000SD51"},
		{FirstName: "Emma", LastName: "Koch", Phone: "+49 152 5554433", IDDocType: domain.DocIDCard, IDDocNumber: "T01000322"},
		{FirstName: "Noah", LastName: "Richter", Email: "noah.richter@example.com", IDDocType: domain.DocPassport, IDDocNumber: "C4J8KK7L2"},
	}
	for i, p := range persons {
		created := must(s.CreatePerson(p, now.AddDate(0, 0, -7)))
		if i < 6 { // 6 von 10 Personen haben bereits einen (Muster-)Ausweis-Scan
			doc := must(domain.NewDocument("ausweis-muster.png", sampleScan(i), now.AddDate(0, 0, -7)))
			must(0, s.SetDocument(created.ID, &doc))
		}
	}

	// Für vergangene Startzeitpunkte wird "now" auf den Startzeitpunkt gesetzt,
	// damit die Regel "Beginn nicht in der Vergangenheit" (K5) eingehalten wird.
	hour := time.Hour
	at := func(d time.Duration) time.Time { return now.Add(d).Truncate(time.Minute) }

	// Zubehör generisch aus den Plugin-Metadaten wählen (C2: kein Plugin-Wissen im Kern):
	// n-tes für den Typ erlaubtes Zubehör, n < 0 = keins.
	extra := func(itemID, n int) []string {
		it := must(s.GetItem(itemID))
		p, ok := s.reg.Get(it.Category)
		if !ok || n < 0 {
			return nil
		}
		var keys []string
		for _, e := range p.Extras() {
			if e.AppliesTo(it.Kind) {
				keys = append(keys, e.Key)
			}
		}
		if len(keys) == 0 {
			return nil
		}
		return []string{keys[n%len(keys)]}
	}

	// 3 laufende Ausleihen (Items 12, 4, 18 → bei Seed-Rädern: E-Bike, Trekkingrad, Kinderrad)
	must(s.CreateRental(12, 1, at(-2*hour), 5, extra(12, 0), at(-2*hour)))
	must(s.CreateRental(4, 2, at(-20*hour), 48, extra(4, 3), at(-20*hour)))
	must(s.CreateRental(18, 3, at(-1*hour), 3, extra(18, 0), at(-1*hour)))
	// 2 Reservierungen
	must(s.CreateRental(16, 4, at(24*hour), 24, extra(16, 4), now))
	must(s.CreateRental(7, 5, at(48*hour), 72, extra(7, -1), now))
	// 1 abgeschlossene Ausleihe (gestern, 4 h geplant, nach 3,5 h zurück)
	r := must(s.CreateRental(1, 6, at(-26*hour), 4, extra(1, 2), at(-26*hour)))
	must(s.ReturnRental(r.ID, at(-26*hour+210*time.Minute)))
}

// sampleScan erzeugt ein kleines Platzhalter-PNG im Ausweisformat (kein echtes Dokument).
func sampleScan(seed int) []byte {
	const w, h = 340, 214
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	base := color.RGBA{R: uint8(90 + seed*20), G: 140, B: 200, A: 255}
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			c := base
			switch {
			case y < 36:
				c = color.RGBA{R: 44, G: 62, B: 80, A: 255}
			case x > 22 && x < 112 && y > 56 && y < 176:
				c = color.RGBA{R: 220, G: 226, B: 232, A: 255}
			case x > 130 && x < 310 && (y-60)%28 < 10 && y > 60 && y < 180:
				c = color.RGBA{R: 240, G: 244, B: 248, A: 255}
			}
			img.Set(x, y, c)
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		log.Fatalf("Muster-Scan: %v", err)
	}
	return buf.Bytes()
}

func must[T any](v T, err error) T {
	if err != nil {
		log.Fatalf("Seed fehlgeschlagen: %v", err)
	}
	return v
}
