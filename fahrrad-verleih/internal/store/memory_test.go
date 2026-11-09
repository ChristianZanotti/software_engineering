package store

import (
	"errors"
	"sync"
	"testing"
	"time"

	"fahrradverleih/internal/domain"
	"fahrradverleih/internal/plugins/bike"
)

var now = time.Date(2026, 6, 1, 9, 0, 0, 0, time.UTC)

func newStore(t *testing.T) *MemoryStore {
	t.Helper()
	s := NewMemoryStore(domain.NewRegistry(bike.Plugin{}))
	items := bike.SampleItems()
	for _, i := range []int{0, 11} { // Citybike (#1), E-Bike (#2)
		if _, err := s.CreateItem(items[i]); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := s.CreatePerson(domain.Person{FirstName: "A", LastName: "B", Phone: "1", IDDocType: domain.DocIDCard, IDDocNumber: "ABCDE1"}, now); err != nil {
		t.Fatal(err)
	}
	return s
}

func TestK5_NoDoubleBooking(t *testing.T) {
	s := newStore(t)
	if _, err := s.CreateRental(1, 1, now, 4, nil, now); err != nil {
		t.Fatal(err)
	}
	if _, err := s.CreateRental(1, 1, now.Add(time.Hour), 1, nil, now); !errors.Is(err, domain.ErrConflict) {
		t.Errorf("Doppelbuchung: err=%v", err)
	}
	if got := len(s.AvailableItems(now, now.Add(time.Hour), now)); got != 1 {
		t.Errorf("verfügbar = %d, want 1", got)
	}
}

func TestK5_ParallelBookingsOnlyOneWins(t *testing.T) {
	s := newStore(t)
	var wg sync.WaitGroup
	var mu sync.Mutex
	ok := 0
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := s.CreateRental(2, 1, now, 3, []string{"helm"}, now); err == nil {
				mu.Lock()
				ok++
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	if ok != 1 {
		t.Errorf("%d erfolgreich, want 1", ok)
	}
}

func TestC3_RentalWithExtras(t *testing.T) {
	s := newStore(t)
	r, err := s.CreateRental(2, 1, now, 3, []string{"helm", "ersatzakku"}, now) // E-Bike 3 × 8 € + 2 € + 6 €
	if err != nil {
		t.Fatal(err)
	}
	if r.PlannedPriceCents != 2400+200+600 || len(r.Extras) != 2 {
		t.Errorf("Preis/Zubehör: %d %+v", r.PlannedPriceCents, r.Extras)
	}
	if _, err := s.CreateRental(1, 1, now, 3, []string{"ersatzakku"}, now); !errors.Is(err, domain.ErrValidation) {
		t.Errorf("Ersatz-Akku für Citybike: err=%v", err)
	}
}

func TestK10_C3_SnapshotSurvivesTariffChange(t *testing.T) {
	s := newStore(t)
	r, _ := s.CreateRental(1, 1, now, 2, []string{"korb"}, now) // 2 × 3 € + 1,50 €
	if _, err := s.UpdateTariff(domain.Tariff{Category: "bike", Kind: "city", HourlyCents: 1000, DailyCents: 5000}); err != nil {
		t.Fatal(err)
	}
	done, err := s.ReturnRental(r.ID, now.Add(2*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if *done.FinalPriceCents != 600+150 {
		t.Errorf("Snapshot verloren: %d", *done.FinalPriceCents)
	}
}

func TestK10_UpdateUnknownTariff(t *testing.T) {
	s := newStore(t)
	if _, err := s.UpdateTariff(domain.Tariff{Category: "bike", Kind: "einrad", HourlyCents: 1, DailyCents: 2}); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("unbekannter Typ: err=%v", err)
	}
}

func TestK2_DeleteProtectionAndCopies(t *testing.T) {
	s := newStore(t)
	_, _ = s.CreateRental(1, 1, now, 2, nil, now)
	if err := s.DeleteItem(1); !errors.Is(err, domain.ErrConflict) {
		t.Errorf("Löschschutz: err=%v", err)
	}
	it, _ := s.GetItem(1)
	it.Attributes["frameSize"] = "XL"
	if again, _ := s.GetItem(1); again.Attributes["frameSize"] == "XL" {
		t.Error("Store darf keine Map-Referenz herausgeben")
	}
}

func TestK4_DuplicateDocument(t *testing.T) {
	s := newStore(t)
	_, err := s.CreatePerson(domain.Person{FirstName: "C", LastName: "D", Phone: "2", IDDocType: domain.DocIDCard, IDDocNumber: "abcde1"}, now)
	if !errors.Is(err, domain.ErrConflict) {
		t.Errorf("doppelter Ausweis: err=%v", err)
	}
}

func TestSeed(t *testing.T) {
	reg := domain.NewRegistry(bike.Plugin{})
	s := NewMemoryStore(reg)
	n := time.Now()
	Seed(s, bike.SampleItems(), n)
	d := domain.BuildDashboard(reg, s.ListItems(), s.ListPersons(), s.ListRentals(), n)
	if d.TotalItems != 20 || d.RentedItems != 3 || d.AvailableItems != 17 || d.Persons != 10 || d.Reservations != 2 {
		t.Errorf("Seed-Dashboard: %+v", d)
	}
}
