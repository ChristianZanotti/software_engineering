package domain

import (
	"fmt"
	"time"
)

// RentalStatus wird immer aus dem Zeitpunkt "jetzt" abgeleitet.
type RentalStatus string

const (
	StatusReserved  RentalStatus = "reserviert"
	StatusActive    RentalStatus = "aktiv"
	StatusOverdue   RentalStatus = "ueberfaellig"
	StatusReturned  RentalStatus = "zurueckgegeben"
	StatusCancelled RentalStatus = "storniert"
)

// Grenzen aus K5.
const (
	MaxRentalHours = 30 * 24
	StartTolerance = 15 * time.Minute
)

// Rental ist eine Ausleihe/Reservierung eines Items (K5, K6) inkl. Tarif- und Zubehör-Snapshot (K10, C3).
type Rental struct {
	ID                int        `json:"id"`
	ItemID            int        `json:"itemId"`
	PersonID          int        `json:"personId"`
	Start             time.Time  `json:"start"`
	DurationHours     int        `json:"durationHours"`
	Tariff            Tariff     `json:"tariff"`
	Extras            []Extra    `json:"extras"`
	PlannedPriceCents int64      `json:"plannedPriceCents"`
	ReturnedAt        *time.Time `json:"returnedAt,omitempty"`
	FinalPriceCents   *int64     `json:"finalPriceCents,omitempty"`
	Cancelled         bool       `json:"cancelled"`
	CreatedAt         time.Time  `json:"createdAt"`
}

// NewRental prüft die Eingaben aus K5 und friert Tarif und Zubehör ein.
func NewRental(itemID, personID int, start time.Time, hours int, tariff Tariff, extras []Extra, now time.Time) (Rental, error) {
	var v validator
	v.check(itemID > 0, "Artikel fehlt")
	v.check(personID > 0, "Person fehlt")
	v.check(!start.IsZero(), "Verleihbeginn fehlt")
	v.check(hours >= 1 && hours <= MaxRentalHours, "Verleihdauer muss zwischen 1 Stunde und 30 Tagen liegen")
	v.check(start.IsZero() || !start.Before(now.Add(-StartTolerance)), "Verleihbeginn darf nicht in der Vergangenheit liegen")
	if err := v.err(); err != nil {
		return Rental{}, err
	}
	if extras == nil {
		extras = []Extra{}
	}
	return Rental{
		ItemID:            itemID,
		PersonID:          personID,
		Start:             start,
		DurationHours:     hours,
		Tariff:            tariff,
		Extras:            extras,
		PlannedPriceCents: tariff.Quote(hours, extras).TotalCents,
		CreatedAt:         now,
	}, nil
}

func (r Rental) PlannedEnd() time.Time {
	return r.Start.Add(time.Duration(r.DurationHours) * time.Hour)
}

// IsOpen: weder zurückgegeben noch storniert – das Item ist belegt.
func (r Rental) IsOpen() bool { return !r.Cancelled && r.ReturnedAt == nil }

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

// IsOut: Item ist gerade beim Kunden (aktiv oder überfällig).
func (r Rental) IsOut(now time.Time) bool {
	s := r.Status(now)
	return s == StatusActive || s == StatusOverdue
}

// Blocks meldet, ob diese Ausleihe [start, end) belegt. Überfällige Items blockieren alles bis zur Rückgabe (K5).
func (r Rental) Blocks(start, end, now time.Time) bool {
	if !r.IsOpen() {
		return false
	}
	if r.Status(now) == StatusOverdue {
		return true
	}
	return start.Before(r.PlannedEnd()) && r.Start.Before(end)
}

// ReturnQuote: Endpreis bei Rückgabe jetzt – tatsächliche Dauer, Snapshot-Tarif, Zubehör pauschal (K9, C3).
func (r Rental) ReturnQuote(now time.Time) Quote {
	return r.Tariff.Quote(BilledHours(now.Sub(r.Start)), r.Extras)
}

// Return bucht die Rückgabe (K6, K9).
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

// Cancel storniert eine Reservierung vor Beginn (K6).
func (r *Rental) Cancel(now time.Time) error {
	if r.Status(now) != StatusReserved {
		return fmt.Errorf("%w: nur Reservierungen vor Beginn können storniert werden (Status: %s)", ErrConflict, r.Status(now))
	}
	r.Cancelled = true
	return nil
}
