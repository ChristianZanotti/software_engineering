package domain

import (
	"sort"
	"time"
)

// Dashboard bündelt die Kennzahlen und Listen der Startseite (A1).
type Dashboard struct {
	TotalSkis            int      `json:"totalSkis"`
	AvailableSkis        int      `json:"availableSkis"`
	RentedSkis           int      `json:"rentedSkis"`
	Persons              int      `json:"persons"`
	PersonsWithoutDoc    int      `json:"personsWithoutDoc"`
	Reservations         int      `json:"reservations"`
	OverdueCount         int      `json:"overdueCount"`
	RevenueTodayCents    int64    `json:"revenueTodayCents"`
	Overdue              []Rental `json:"-"`
	DueToday             []Rental `json:"-"`
	UpcomingReservations []Rental `json:"-"`
}

// BuildDashboard berechnet das Dashboard aus den aktuellen Daten (A1).
func BuildDashboard(skis []Ski, persons []Person, rentals []Rental, now time.Time) Dashboard {
	d := Dashboard{TotalSkis: len(skis), Persons: len(persons)}
	for _, p := range persons {
		if p.Document == nil {
			d.PersonsWithoutDoc++
		}
	}

	out := map[int]bool{}
	y, m, day := now.Date()
	startOfDay := time.Date(y, m, day, 0, 0, 0, 0, now.Location())
	endOfDay := startOfDay.AddDate(0, 0, 1)

	for _, r := range rentals {
		switch r.Status(now) {
		case StatusActive:
			out[r.SkiID] = true
			if r.PlannedEnd().Before(endOfDay) {
				d.DueToday = append(d.DueToday, r)
			}
		case StatusOverdue:
			out[r.SkiID] = true
			d.Overdue = append(d.Overdue, r)
		case StatusReserved:
			d.Reservations++
			d.UpcomingReservations = append(d.UpcomingReservations, r)
		case StatusReturned:
			if !r.ReturnedAt.Before(startOfDay) && r.FinalPriceCents != nil {
				d.RevenueTodayCents += *r.FinalPriceCents
			}
		}
	}
	d.RentedSkis = len(out)
	d.AvailableSkis = d.TotalSkis - d.RentedSkis
	d.OverdueCount = len(d.Overdue)

	byEnd := func(list []Rental) {
		sort.Slice(list, func(i, j int) bool { return list[i].PlannedEnd().Before(list[j].PlannedEnd()) })
	}
	byEnd(d.Overdue)
	byEnd(d.DueToday)
	sort.Slice(d.UpcomingReservations, func(i, j int) bool {
		return d.UpcomingReservations[i].Start.Before(d.UpcomingReservations[j].Start)
	})
	if len(d.UpcomingReservations) > 5 {
		d.UpcomingReservations = d.UpcomingReservations[:5]
	}
	return d
}
