package domain

import "time"

// Tariff ist der Preis für einen Typ einer Kategorie (K8, K10).
type Tariff struct {
	Category    string `json:"category"`
	Kind        string `json:"kind"`
	HourlyCents int64  `json:"hourlyCents"`
	DailyCents  int64  `json:"dailyCents"`
}

// TariffKey bildet den Schlüssel "<category>/<kind>".
func TariffKey(category, kind string) string { return category + "/" + kind }

func (t Tariff) Key() string { return TariffKey(t.Category, t.Kind) }

// Validate: beide Preise > 0, Tagespreis ≥ Stundenpreis.
func (t Tariff) Validate() error {
	var v validator
	v.check(t.HourlyCents > 0, "Stundenpreis muss größer 0 sein")
	v.check(t.DailyCents > 0, "Tagespreis muss größer 0 sein")
	v.check(t.DailyCents >= t.HourlyCents, "Tagespreis muss mindestens so hoch wie der Stundenpreis sein")
	return v.err()
}

// Quote ist ein Preis inklusive Rechenweg und Zubehör (K8, C6).
type Quote struct {
	Hours       int     `json:"hours"`
	FullDays    int     `json:"fullDays"`
	RestHours   int     `json:"restHours"`
	DaysCents   int64   `json:"daysCents"`
	RestCents   int64   `json:"restCents"`
	RestCapped  bool    `json:"restCapped"`
	RentCents   int64   `json:"rentCents"`
	Extras      []Extra `json:"extras"`
	ExtrasCents int64   `json:"extrasCents"`
	TotalCents  int64   `json:"totalCents"`
	HourlyCents int64   `json:"hourlyCents"`
	DailyCents  int64   `json:"dailyCents"`
}

// Quote berechnet den Mietpreis stundenweise mit Tagesdeckel (K8) und addiert Zubehör pauschal (C3):
//
//	Miete = volleTage × Tagespreis + min(restStd × Stundenpreis, Tagespreis)
//	Gesamt = Miete + Σ Zubehör
func (t Tariff) Quote(hours int, extras []Extra) Quote {
	q := Quote{Hours: hours, HourlyCents: t.HourlyCents, DailyCents: t.DailyCents, Extras: extras}
	if q.Extras == nil {
		q.Extras = []Extra{}
	}
	if hours > 0 {
		q.FullDays = hours / 24
		q.RestHours = hours % 24
		q.DaysCents = int64(q.FullDays) * t.DailyCents
		q.RestCents = int64(q.RestHours) * t.HourlyCents
		if q.RestCents > t.DailyCents {
			q.RestCents = t.DailyCents
			q.RestCapped = true
		}
		q.RentCents = q.DaysCents + q.RestCents
	}
	for _, e := range extras {
		q.ExtrasCents += e.PriceCents
	}
	q.TotalCents = q.RentCents + q.ExtrasCents
	return q
}

// BilledHours rundet auf angefangene Stunden auf, mindestens 1 (K9).
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
