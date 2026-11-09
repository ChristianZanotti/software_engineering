package store

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"log"
	"time"

	"skiverleih/internal/domain"
)

// Seed lädt die Beispieldaten aus A8. Es werden nur öffentliche Store-Methoden benutzt,
// d. h. alle Daten durchlaufen dieselben Prüfungen wie echte Eingaben.
func Seed(s *MemoryStore, now time.Time) {
	skis := []domain.Ski{
		{Brand: "Atomic", Model: "Redster S9", Type: domain.SkiCarving, LengthCm: 165, Condition: domain.ConditionNew},
		{Brand: "Atomic", Model: "Redster S9", Type: domain.SkiCarving, LengthCm: 170, Condition: domain.ConditionGood},
		{Brand: "Head", Model: "Supershape e-Rally", Type: domain.SkiCarving, LengthCm: 163, Condition: domain.ConditionGood},
		{Brand: "Rossignol", Model: "Experience 80", Type: domain.SkiAlpin, LengthCm: 168, Condition: domain.ConditionGood},
		{Brand: "Rossignol", Model: "Experience 80", Type: domain.SkiAlpin, LengthCm: 176, Condition: domain.ConditionUsed},
		{Brand: "Völkl", Model: "Deacon 76", Type: domain.SkiAlpin, LengthCm: 158, Condition: domain.ConditionNew},
		{Brand: "Fischer", Model: "RC One 72", Type: domain.SkiAlpin, LengthCm: 150, Condition: domain.ConditionGood},
		{Brand: "Salomon", Model: "Stance 90", Type: domain.SkiAlpin, LengthCm: 182, Condition: domain.ConditionGood},
		{Brand: "Elan", Model: "Primetime 33", Type: domain.SkiAlpin, LengthCm: 120, Condition: domain.ConditionUsed, Notes: "Kinderski"},
		{Brand: "K2", Model: "Mindbender 99", Type: domain.SkiFreeride, LengthCm: 177, Condition: domain.ConditionNew},
		{Brand: "Black Crows", Model: "Atris", Type: domain.SkiFreeride, LengthCm: 184, Condition: domain.ConditionGood},
		{Brand: "Faction", Model: "Dancer 2", Type: domain.SkiFreeride, LengthCm: 178, Condition: domain.ConditionGood},
		{Brand: "Fischer", Model: "Twin Skin Pro", Type: domain.SkiLanglauf, LengthCm: 192, Condition: domain.ConditionGood},
		{Brand: "Madshus", Model: "Endurace", Type: domain.SkiLanglauf, LengthCm: 197, Condition: domain.ConditionNew},
		{Brand: "Rossignol", Model: "Evo XT 55", Type: domain.SkiLanglauf, LengthCm: 185, Condition: domain.ConditionUsed},
		{Brand: "Dynafit", Model: "Blacklight 88", Type: domain.SkiTouren, LengthCm: 172, Condition: domain.ConditionGood},
		{Brand: "Ski Trab", Model: "Maestro.2", Type: domain.SkiTouren, LengthCm: 171, Condition: domain.ConditionNew},
		{Brand: "Burton", Model: "Custom", Type: domain.SkiSnowboard, LengthCm: 156, Condition: domain.ConditionGood},
		{Brand: "Burton", Model: "Process", Type: domain.SkiSnowboard, LengthCm: 152, Condition: domain.ConditionUsed},
		{Brand: "Nitro", Model: "Team", Type: domain.SkiSnowboard, LengthCm: 159, Condition: domain.ConditionNew},
	}
	for _, k := range skis {
		must(s.CreateSki(k))
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
	// damit die Regel "Beginn nicht in der Vergangenheit" (A5) eingehalten wird.
	hour := time.Hour
	at := func(d time.Duration) time.Time { return now.Add(d).Truncate(time.Minute) }

	// 3 laufende Ausleihen
	must(s.CreateRental(4, 1, at(-2*hour), 5, at(-2*hour)))
	must(s.CreateRental(10, 2, at(-20*hour), 48, at(-20*hour)))
	must(s.CreateRental(18, 3, at(-1*hour), 3, at(-1*hour)))
	// 2 Reservierungen
	must(s.CreateRental(1, 4, at(24*hour), 24, now))
	must(s.CreateRental(16, 5, at(48*hour), 72, now))
	// 1 abgeschlossene Ausleihe (gestern, 4 h geplant, nach 3,5 h zurück)
	r := must(s.CreateRental(13, 6, at(-26*hour), 4, at(-26*hour)))
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
