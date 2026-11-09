package domain

import (
	"fmt"
	"time"
)

// RentalStatus wird immer aus dem Zeitpunkt "jetzt" abgeleitet (siehe README, Glossar).
type RentalStatus string

const (
	StatusReserved  RentalStatus = "reserviert"
	StatusActive    RentalStatus = "aktiv"
	StatusOverdue   RentalStatus = "ueberfaellig"
	StatusReturned  RentalStatus = "zurueckgegeben"
	StatusCancelled RentalStatus = "storniert"
)

// Grenzen aus A5.
const (
	MaxRentalHours = 30 * 24
	StartTolerance = 15 * time.Minute
)

// Rental ist eine Ausleihe bzw. Reservierung (A5, A6) inkl. Tarif-Snapshot (B6).
type Rental struct {
	ID                int        `json:"id"`
	SkiID             int        `json:"skiId"`
	PersonID          int        `json:"personId"`
	Start             time.Time  `json:"start"`
	DurationHours     int        `json:"durationHours"`
	Tariff            Tariff     `json:"tariff"`
	PlannedPriceCents int64      `json:"plannedPriceCents"`
	ReturnedAt        *time.Time `json:"returnedAt,omitempty"`
	FinalPriceCents   *int64     `json:"finalPriceCents,omitempty"`
	Cancelled         bool       `json:"cancelled"`
	CreatedAt         time.Time  `json:"createdAt"`
}

// NewRental prüft die Eingaben aus A5 und friert den Tarif ein (B6).
func NewRental(skiID, personID int, start time.Time, hours int, tariff Tariff, now time.Time) (Rental, error) {
	var v validator
	v.check(skiID > 0, "Ski fehlt")
	v.check(personID > 0, "Person fehlt")
	v.check(!start.IsZero(), "Verleihbeginn fehlt")
	v.check(hours >= 1 && hours <= MaxRentalHours, "Verleihdauer muss zwischen 1 Stunde und 30 Tagen liegen")
	v.check(start.IsZero() || !start.Before(now.Add(-StartTolerance)), "Verleihbeginn darf nicht in der Vergangenheit liegen")
	if err := v.err(); err != nil {
		return Rental{}, err
	}
	return Rental{
		SkiID:             skiID,
		PersonID:          personID,
		Start:             start,
		DurationHours:     hours,
		Tariff:            tariff,
		PlannedPriceCents: tariff.Price(hours),
		CreatedAt:         now,
	}, nil
}

// PlannedEnd ist Beginn + Verleihdauer.
func (r Rental) PlannedEnd() time.Time {
	return r.Start.Add(time.Duration(r.DurationHours) * time.Hour)
}

// IsOpen: weder zurückgegeben noch storniert – der Ski ist damit belegt.
func (r Rental) IsOpen() bool { return !r.Cancelled && r.ReturnedAt == nil }

// Status leitet den Status aus "jetzt" ab.
func (r Rental) Status(now time.Time) RentalStatus {
	switch {
	case r.Cancelled:
		return StatusCancelled
	case r.ReturnedAt != nil:
		return StatusReturned
	case now.Before(r.Start):
		return StatusReserved
	case now.Before(r.PlannedEnd()):
		return StatusActive
	default:
		return StatusOverdue
	}
}

// IsOut meldet, ob der Ski gerade physisch beim Kunden ist (aktiv oder überfällig).
func (r Rental) IsOut(now time.Time) bool {
	s := r.Status(now)
	return s == StatusActive || s == StatusOverdue
}

// Blocks meldet, ob diese Ausleihe den Zeitraum [start, end) für ihren Ski belegt (A5).
// Ein überfälliger Ski blockiert jeden neuen Zeitraum, bis er zurückgegeben ist.
func (r Rental) Blocks(start, end, now time.Time) bool {
	if !r.IsOpen() {
		return false
	}
	if r.Status(now) == StatusOverdue {
		return true
	}
	return start.Before(r.PlannedEnd()) && r.Start.Before(end)
}

// ReturnQuote berechnet den Endpreis, falls jetzt zurückgegeben würde:
// angefangene Stunden seit Beginn, Tarif-Snapshot der Buchung (B5, B6).
func (r Rental) ReturnQuote(now time.Time) Quote {
	return r.Tariff.Quote(BilledHours(now.Sub(r.Start)))
}

// Return bucht die Rückgabe und rechnet nach tatsächlicher Dauer ab (A6, B5).
func (r *Rental) Return(now time.Time) error {
	if !r.IsOut(now) {
		return fmt.Errorf("%w: nur aktive oder überfällige Ausleihen können zurückgegeben werden (Status: %s)", ErrConflict, r.Status(now))
	}
	t := now
	price := r.ReturnQuote(now).TotalCents
	r.ReturnedAt = &t
	r.FinalPriceCents = &price
	return nil
}

// Cancel storniert eine Reservierung vor Beginn (A6).
func (r *Rental) Cancel(now time.Time) error {
	if r.Status(now) != StatusReserved {
		return fmt.Errorf("%w: nur Reservierungen vor Beginn können storniert werden (Status: %s)", ErrConflict, r.Status(now))
	}
	r.Cancelled = true
	return nil
}
