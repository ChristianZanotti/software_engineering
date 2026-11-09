package store

import (
	"errors"
	"sync"
	"testing"
	"time"

	"skiverleih/internal/domain"
)

var now = time.Date(2026, 12, 1, 9, 0, 0, 0, time.UTC)

func newStore(t *testing.T) *MemoryStore {
	t.Helper()
	s := NewMemoryStore()
	if _, err := s.CreateSki(domain.Ski{Brand: "Atomic", Model: "X", Type: domain.SkiAlpin, LengthCm: 170, Condition: domain.ConditionGood}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.CreatePerson(domain.Person{FirstName: "A", LastName: "B", Phone: "1", IDDocType: domain.DocIDCard, IDDocNumber: "ABCDE1"}, now); err != nil {
		t.Fatal(err)
	}
	return s
}

func TestA5_NoDoubleBooking(t *testing.T) {
	s := newStore(t)
	if _, err := s.CreateRental(1, 1, now, 4, now); err != nil {
		t.Fatal(err)
	}
	if _, err := s.CreateRental(1, 1, now.Add(2*time.Hour), 4, now); !errors.Is(err, domain.ErrConflict) {
		t.Errorf("Überschneidung muss Konflikt sein, err=%v", err)
	}
	if _, err := s.CreateRental(1, 1, now.Add(4*time.Hour), 4, now); err != nil {
		t.Errorf("direkt anschließende Ausleihe muss erlaubt sein: %v", err)
	}
	if got := len(s.AvailableSkis(now.Add(time.Hour), now.Add(2*time.Hour), now)); got != 0 {
		t.Errorf("AvailableSkis = %d, want 0", got)
	}
	if got := len(s.AvailableSkis(now.Add(9*time.Hour), now.Add(10*time.Hour), now)); got != 1 {
		t.Errorf("AvailableSkis später = %d, want 1", got)
	}
}

func TestA5_ParallelBookingsOnlyOneWins(t *testing.T) {
	s := newStore(t)
	var wg sync.WaitGroup
	var mu sync.Mutex
	ok := 0
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := s.CreateRental(1, 1, now, 3, now); err == nil {
				mu.Lock()
				ok++
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	if ok != 1 {
		t.Errorf("%d parallele Buchungen erfolgreich, want 1", ok)
	}
}

func TestA5_UnknownSkiOrPerson(t *testing.T) {
	s := newStore(t)
	if _, err := s.CreateRental(99, 1, now, 1, now); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("unbekannter Ski: err=%v", err)
	}
	if _, err := s.CreateRental(1, 99, now, 1, now); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("unbekannte Person: err=%v", err)
	}
}

func TestA2_A3_DeleteProtection(t *testing.T) {
	s := newStore(t)
	r, _ := s.CreateRental(1, 1, now, 2, now)
	if err := s.DeleteSki(1); !errors.Is(err, domain.ErrConflict) {
		t.Errorf("Ski mit offener Ausleihe gelöscht, err=%v", err)
	}
	if err := s.DeletePerson(1); !errors.Is(err, domain.ErrConflict) {
		t.Errorf("Person mit offener Ausleihe gelöscht, err=%v", err)
	}
	if _, err := s.ReturnRental(r.ID, now.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	if err := s.DeleteSki(1); err != nil {
		t.Errorf("Löschen nach Rückgabe: %v", err)
	}
}

func TestA4_DuplicateDocument(t *testing.T) {
	s := newStore(t)
	_, err := s.CreatePerson(domain.Person{FirstName: "C", LastName: "D", Phone: "2", IDDocType: domain.DocIDCard, IDDocNumber: "abcde1"}, now)
	if !errors.Is(err, domain.ErrConflict) {
		t.Errorf("doppelter Ausweis muss Konflikt sein, err=%v", err)
	}
	if _, err := s.CreatePerson(domain.Person{FirstName: "C", LastName: "D", Phone: "2", IDDocType: domain.DocPassport, IDDocNumber: "ABCDE1"}, now); err != nil {
		t.Errorf("gleiche Nummer, anderer Typ muss erlaubt sein: %v", err)
	}
}

func TestA4_UpdateKeepsDocument(t *testing.T) {
	s := newStore(t)
	doc, _ := domain.NewDocument("a.pdf", []byte("%PDF-1.4"), now)
	if err := s.SetDocument(1, &doc); err != nil {
		t.Fatal(err)
	}
	p, _ := s.GetPerson(1)
	p.Phone = "999"
	p.Document = nil
	if _, err := s.UpdatePerson(p); err != nil {
		t.Fatal(err)
	}
	if got, _ := s.GetPerson(1); got.Document == nil || got.Phone != "999" {
		t.Errorf("Update hat Dokument verloren oder nicht gespeichert: %+v", got)
	}
}

func TestB5_TariffChangeDoesNotAffectExisting(t *testing.T) {
	s := newStore(t)
	r, _ := s.CreateRental(1, 1, now, 3, now) // 3 × 8 € = 24 €
	if _, err := s.UpdateTariff(domain.Tariff{SkiType: domain.SkiAlpin, HourlyCents: 2000, DailyCents: 9000}); err != nil {
		t.Fatal(err)
	}
	done, err := s.ReturnRental(r.ID, now.Add(3*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if done.PlannedPriceCents != 2400 || *done.FinalPriceCents != 2400 {
		t.Errorf("alter Tarif nicht beibehalten: geplant %d, final %d", done.PlannedPriceCents, *done.FinalPriceCents)
	}
	r2, _ := s.CreateRental(1, 1, now.Add(5*time.Hour), 3, now)
	if r2.PlannedPriceCents != 6000 {
		t.Errorf("neuer Tarif nicht verwendet: %d", r2.PlannedPriceCents)
	}
}

func TestA8_Seed(t *testing.T) {
	s := NewMemoryStore()
	n := time.Now()
	Seed(s, n)
	if len(s.ListSkis()) != 20 || len(s.ListPersons()) != 10 {
		t.Fatalf("Seed-Mengen falsch")
	}
	d := domain.BuildDashboard(s.ListSkis(), s.ListPersons(), s.ListRentals(), n)
	if d.RentedSkis != 3 || d.AvailableSkis != 17 || d.Reservations != 2 {
		t.Errorf("Seed-Dashboard: %+v", d)
	}
}
