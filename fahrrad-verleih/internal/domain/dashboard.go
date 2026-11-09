package domain

import (
	"sort"
	"time"
)

// KindStat ist die Verfügbarkeit eines Typs (C5).
type KindStat struct {
	Category  string `json:"category"`
	Kind      string `json:"kind"`
	Label     string `json:"label"`
	Total     int    `json:"total"`
	Available int    `json:"available"`
}

// Dashboard bündelt Kennzahlen und Listen der Startseite (K1, C5).
type Dashboard struct {
	TotalItems           int        `json:"totalItems"`
	AvailableItems       int        `json:"availableItems"`
	RentedItems          int        `json:"rentedItems"`
	Persons              int        `json:"persons"`
	PersonsWithoutDoc    int        `json:"personsWithoutDoc"`
	Reservations         int        `json:"reservations"`
	OverdueCount         int        `json:"overdueCount"`
	RevenueTodayCents    int64      `json:"revenueTodayCents"`
	ByKind               []KindStat `json:"byKind"`
	Overdue              []Rental   `json:"-"`
	DueToday             []Rental   `json:"-"`
	UpcomingReservations []Rental   `json:"-"`
}

// BuildDashboard berechnet das Dashboard. Die Typ-Reihenfolge kommt aus der Registry.
func BuildDashboard(reg *Registry, items []Item, persons []Person, rentals []Rental, now time.Time) Dashboard {
	d := Dashboard{TotalItems: len(items), Persons: len(persons)}
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
			out[r.ItemID] = true
			if r.PlannedEnd().Before(endOfDay) {
				d.DueToday = append(d.DueToday, r)
			}
		case StatusOverdue:
			out[r.ItemID] = true
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
	d.RentedItems = len(out)
	d.AvailableItems = d.TotalItems - d.RentedItems
	d.OverdueCount = len(d.Overdue)

	// C5: Verfügbarkeit pro Typ
	stats := map[string]*KindStat{}
	d.ByKind = []KindStat{}
	for _, p := range reg.All() {
		for _, k := range p.Kinds() {
			d.ByKind = append(d.ByKind, KindStat{Category: p.Category(), Kind: k.Key, Label: k.Label})
		}
	}
	for i := range d.ByKind {
		stats[TariffKey(d.ByKind[i].Category, d.ByKind[i].Kind)] = &d.ByKind[i]
	}
	for _, it := range items {
		if s, ok := stats[TariffKey(it.Category, it.Kind)]; ok {
			s.Total++
			if !out[it.ID] {
				s.Available++
			}
		}
	}

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
