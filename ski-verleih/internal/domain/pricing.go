package domain

import "time"

// Tariff ist der Preis für einen Ski-Typ (B2): Stundenpreis + Tagespreis als Tagesdeckel.
type Tariff struct {
	SkiType     SkiType `json:"skiType"`
	HourlyCents int64   `json:"hourlyCents"`
	DailyCents  int64   `json:"dailyCents"`
}

// Validate prüft die Akzeptanzkriterien aus B2.
func (t Tariff) Validate() error {
	var v validator
	v.check(t.SkiType.Valid(), "Ski-Typ ist ungültig")
	v.check(t.HourlyCents > 0, "Stundenpreis muss größer 0 sein")
	v.check(t.DailyCents > 0, "Tagespreis muss größer 0 sein")
	v.check(t.DailyCents >= t.HourlyCents, "Tagespreis muss mindestens so hoch wie der Stundenpreis sein")
	return v.err()
}

// Quote ist ein Preis inklusive Rechenweg (B3, B4).
type Quote struct {
	Hours       int   `json:"hours"`
	FullDays    int   `json:"fullDays"`
	RestHours   int   `json:"restHours"`
	DaysCents   int64 `json:"daysCents"`
	RestCents   int64 `json:"restCents"`
	RestCapped  bool  `json:"restCapped"`
	TotalCents  int64 `json:"totalCents"`
	HourlyCents int64 `json:"hourlyCents"`
	DailyCents  int64 `json:"dailyCents"`
}

// Quote berechnet den Preis stundenweise mit Tagesdeckel (B3):
//
//	Preis = volleTage × Tagespreis + min(restStd × Stundenpreis, Tagespreis)
func (t Tariff) Quote(hours int) Quote {
	q := Quote{Hours: hours, HourlyCents: t.HourlyCents, DailyCents: t.DailyCents}
	if hours <= 0 {
		return q
	}
	q.FullDays = hours / 24
	q.RestHours = hours % 24
	q.DaysCents = int64(q.FullDays) * t.DailyCents
	q.RestCents = int64(q.RestHours) * t.HourlyCents
	if q.RestCents > t.DailyCents {
		q.RestCents = t.DailyCents
		q.RestCapped = true
	}
	q.TotalCents = q.DaysCents + q.RestCents
	return q
}

// Price ist die Kurzform von Quote(hours).TotalCents.
func (t Tariff) Price(hours int) int64 { return t.Quote(hours).TotalCents }

// BilledHours rundet eine tatsächliche Nutzungsdauer auf angefangene Stunden auf, mindestens 1 (B5).
func BilledHours(d time.Duration) int {
	if d <= 0 {
		return 1
	}
	h := int(d / time.Hour)
	if d%time.Hour != 0 {
		h++
	}
	return h
}

// DefaultTariffs liefert die Starttarife aus der Spezifikation (B2).
func DefaultTariffs() map[SkiType]Tariff {
	cents := map[SkiType][2]int64{
		SkiAlpin:     {800, 3500},
		SkiCarving:   {900, 3900},
		SkiFreeride:  {1100, 4900},
		SkiLanglauf:  {500, 2200},
		SkiTouren:    {1000, 4500},
		SkiSnowboard: {800, 3500},
	}
	out := make(map[SkiType]Tariff, len(cents))
	for t, c := range cents {
		out[t] = Tariff{SkiType: t, HourlyCents: c[0], DailyCents: c[1]}
	}
	return out
}
