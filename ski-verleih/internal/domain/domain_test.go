package domain

import (
	"errors"
	"testing"
	"time"
)

var t0 = time.Date(2026, 12, 1, 9, 0, 0, 0, time.UTC)

func alpin() Tariff { return DefaultTariffs()[SkiAlpin] }

// B3: Akzeptanzbeispiele aus dem README.
func TestB2_PriceCalculation(t *testing.T) {
	cases := []struct {
		hours int
		want  int64
	}{
		{1, 800}, {3, 2400}, {5, 3500}, {24, 3500}, {26, 5100}, {30, 7000}, {48, 7000}, {0, 0},
	}
	for _, c := range cases {
		if got := alpin().Price(c.hours); got != c.want {
			t.Errorf("Price(%d h) = %d, want %d", c.hours, got, c.want)
		}
	}
}

func TestB2_QuoteBreakdown(t *testing.T) {
	q := alpin().Quote(30)
	if q.FullDays != 1 || q.RestHours != 6 || !q.RestCapped || q.RestCents != 3500 || q.TotalCents != 7000 {
		t.Errorf("unerwarteter Rechenweg: %+v", q)
	}
}

func TestB1_TariffValidation(t *testing.T) {
	if err := (Tariff{SkiType: SkiAlpin, HourlyCents: 1000, DailyCents: 500}).Validate(); !errors.Is(err, ErrValidation) {
		t.Errorf("Tagespreis < Stundenpreis muss abgelehnt werden, err=%v", err)
	}
	if err := (Tariff{SkiType: "quatsch", HourlyCents: 100, DailyCents: 500}).Validate(); err == nil {
		t.Error("ungültiger Typ muss abgelehnt werden")
	}
	if err := alpin().Validate(); err != nil {
		t.Errorf("Standardtarif ungültig: %v", err)
	}
}

// B5: angefangene Stunden, mindestens 1.
func TestB4_BilledHours(t *testing.T) {
	cases := map[time.Duration]int{
		0: 1, time.Minute: 1, time.Hour: 1, time.Hour + time.Second: 2, 210 * time.Minute: 4, 24 * time.Hour: 24,
	}
	for d, want := range cases {
		if got := BilledHours(d); got != want {
			t.Errorf("BilledHours(%v) = %d, want %d", d, got, want)
		}
	}
}

func newRental(t *testing.T, start time.Time, hours int) Rental {
	t.Helper()
	r, err := NewRental(1, 1, start, hours, alpin(), start)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

// A5: Eingabegrenzen.
func TestA5_NewRentalValidation(t *testing.T) {
	cases := []struct {
		name  string
		start time.Time
		hours int
	}{
		{"Dauer 0", t0, 0},
		{"Dauer > 30 Tage", t0, MaxRentalHours + 1},
		{"Beginn zu weit in der Vergangenheit", t0.Add(-time.Hour), 3},
		{"Beginn fehlt", time.Time{}, 3},
	}
	for _, c := range cases {
		if _, err := NewRental(1, 1, c.start, c.hours, alpin(), t0); !errors.Is(err, ErrValidation) {
			t.Errorf("%s: erwartet Validierungsfehler, err=%v", c.name, err)
		}
	}
	if _, err := NewRental(1, 1, t0.Add(-10*time.Minute), 3, alpin(), t0); err != nil {
		t.Errorf("10 Minuten Toleranz muss erlaubt sein: %v", err)
	}
}

// B6: Tarif wird eingefroren, Preis bei Buchung berechnet.
func TestB5_TariffSnapshot(t *testing.T) {
	r := newRental(t, t0, 26)
	if r.Tariff != alpin() || r.PlannedPriceCents != 5100 {
		t.Errorf("Snapshot/Preis falsch: %+v", r)
	}
}

func TestA6_StatusLifecycle(t *testing.T) {
	r := newRental(t, t0, 3)
	checks := []struct {
		at   time.Time
		want RentalStatus
	}{
		{t0.Add(-time.Minute), StatusReserved},
		{t0, StatusActive},
		{t0.Add(3*time.Hour - time.Second), StatusActive},
		{t0.Add(3 * time.Hour), StatusOverdue},
	}
	for _, c := range checks {
		if got := r.Status(c.at); got != c.want {
			t.Errorf("Status(%v) = %s, want %s", c.at, got, c.want)
		}
	}
}

// A6 + B5: Rückgabe rechnet nach tatsächlicher Dauer ab.
func TestA6_B4_ReturnChargesActualDuration(t *testing.T) {
	early := newRental(t, t0, 5) // geplant 35 €
	if err := early.Return(t0.Add(90 * time.Minute)); err != nil {
		t.Fatal(err)
	}
	if *early.FinalPriceCents != 1600 { // 2 angefangene Stunden
		t.Errorf("früh zurück: %d, want 1600", *early.FinalPriceCents)
	}
	if early.Status(t0.Add(10*time.Hour)) != StatusReturned {
		t.Error("Status muss zurueckgegeben sein")
	}

	late := newRental(t, t0, 2)
	if err := late.Return(t0.Add(26*time.Hour + time.Minute)); err != nil {
		t.Fatal(err)
	}
	if *late.FinalPriceCents != 3500+3*800 { // 27 angefangene Stunden
		t.Errorf("spät zurück: %d", *late.FinalPriceCents)
	}

	reserved := newRental(t, t0, 2)
	if err := reserved.Return(t0.Add(-time.Hour)); !errors.Is(err, ErrConflict) {
		t.Errorf("Rückgabe einer Reservierung muss Konflikt sein, err=%v", err)
	}
}

func TestA6_CancelOnlyReservations(t *testing.T) {
	r := newRental(t, t0, 2)
	if err := r.Cancel(t0.Add(time.Minute)); !errors.Is(err, ErrConflict) {
		t.Errorf("aktive Ausleihe darf nicht storniert werden, err=%v", err)
	}
	if err := r.Cancel(t0.Add(-time.Minute)); err != nil || r.Status(t0) != StatusCancelled || r.IsOpen() {
		t.Errorf("Stornierung fehlgeschlagen: err=%v status=%s", err, r.Status(t0))
	}
}

// A5: Überschneidungen halboffener Intervalle, überfällige Ski blockieren alles.
func TestA5_Blocks(t *testing.T) {
	r := newRental(t, t0, 4) // 9–13 Uhr
	now := t0.Add(-time.Hour)
	h := time.Hour
	cases := []struct {
		name       string
		start, end time.Time
		want       bool
	}{
		{"komplett davor", t0.Add(-3 * h), t0, false},
		{"direkt danach", t0.Add(4 * h), t0.Add(6 * h), false},
		{"überlappt Anfang", t0.Add(-h), t0.Add(h), true},
		{"liegt innen", t0.Add(h), t0.Add(2 * h), true},
		{"umschließt", t0.Add(-h), t0.Add(5 * h), true},
	}
	for _, c := range cases {
		if got := r.Blocks(c.start, c.end, now); got != c.want {
			t.Errorf("%s: Blocks = %v, want %v", c.name, got, c.want)
		}
	}
	overdueNow := t0.Add(5 * h)
	if !r.Blocks(t0.Add(48*h), t0.Add(50*h), overdueNow) {
		t.Error("überfälliger Ski muss jeden Zeitraum blockieren")
	}
}

// A3 + A4: Pflichtfelder und Ausweisnummer.
func TestA4_PersonValidation(t *testing.T) {
	ok := Person{FirstName: "Anna", LastName: "Huber", Email: "a@b.de", IDDocType: DocIDCard, IDDocNumber: "l01x 00t47"}
	ok.Normalize()
	if ok.IDDocNumber != "L01X00T47" {
		t.Errorf("Normalisierung: %q", ok.IDDocNumber)
	}
	if err := ok.Validate(); err != nil {
		t.Errorf("gültige Person abgelehnt: %v", err)
	}
	bad := Person{FirstName: "Anna"}
	err := bad.Validate()
	var ve *ValidationError
	if !errors.As(err, &ve) || len(ve.Problems) < 4 {
		t.Errorf("erwartet mehrere Probleme, got %v", err)
	}
}

func TestA4_DocumentContentCheck(t *testing.T) {
	png := []byte("\x89PNG\r\n\x1a\n\x00\x00\x00\rIHDR")
	if d, err := NewDocument("ausweis.png", png, t0); err != nil || d.ContentType != "image/png" {
		t.Errorf("PNG abgelehnt: %v", err)
	}
	pdf := []byte("%PDF-1.4\n...")
	if _, err := NewDocument("ausweis.pdf", pdf, t0); err != nil {
		t.Errorf("PDF abgelehnt: %v", err)
	}
	if _, err := NewDocument("ausweis.png", []byte("<html>hallo</html>"), t0); !errors.Is(err, ErrValidation) {
		t.Error("HTML mit .png-Endung muss abgelehnt werden")
	}
	big := append([]byte("%PDF-1.4\n"), make([]byte, MaxDocumentBytes)...)
	if _, err := NewDocument("gross.pdf", big, t0); !errors.Is(err, ErrValidation) {
		t.Error("Datei > 5 MB muss abgelehnt werden")
	}
}

func TestA1_Dashboard(t *testing.T) {
	skis := []Ski{{ID: 1}, {ID: 2}, {ID: 3}}
	persons := []Person{{ID: 1}, {ID: 2, Document: &Document{}}}
	now := t0.Add(2 * time.Hour)
	active, _ := NewRental(1, 1, t0, 4, alpin(), t0)
	overdue, _ := NewRental(2, 1, t0.Add(-5*time.Hour), 1, alpin(), t0.Add(-5*time.Hour))
	reserved, _ := NewRental(3, 2, t0.Add(5*time.Hour), 1, alpin(), t0)
	returned, _ := NewRental(3, 2, t0, 1, alpin(), t0)
	_ = returned.Return(t0.Add(30 * time.Minute))

	d := BuildDashboard(skis, persons, []Rental{active, overdue, reserved, returned}, now)
	if d.TotalSkis != 3 || d.RentedSkis != 2 || d.AvailableSkis != 1 || d.Persons != 2 || d.PersonsWithoutDoc != 1 {
		t.Errorf("Kennzahlen falsch: %+v", d)
	}
	if d.Reservations != 1 || d.OverdueCount != 1 || len(d.DueToday) != 1 || d.RevenueTodayCents != 800 {
		t.Errorf("Listen/Umsatz falsch: %+v", d)
	}
}
