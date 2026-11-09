// Package store hält alle Daten thread-safe im Speicher.
// Konfliktprüfungen (Doppelbuchung, Löschschutz) laufen atomar unter dem Lock.
package store

import (
	"fmt"
	"sort"
	"sync"
	"time"

	"skiverleih/internal/domain"
)

// MemoryStore ist der In-Memory-Speicher. Er gibt immer Kopien heraus.
type MemoryStore struct {
	mu         sync.RWMutex
	skis       map[int]domain.Ski
	persons    map[int]domain.Person
	rentals    map[int]domain.Rental
	tariffs    map[domain.SkiType]domain.Tariff
	nextSki    int
	nextPerson int
	nextRental int
}

// NewMemoryStore legt einen leeren Store mit Standardtarifen an.
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		skis:       map[int]domain.Ski{},
		persons:    map[int]domain.Person{},
		rentals:    map[int]domain.Rental{},
		tariffs:    domain.DefaultTariffs(),
		nextSki:    1,
		nextPerson: 1,
		nextRental: 1,
	}
}

func notFound(what string, id int) error {
	return fmt.Errorf("%w: %s %d", domain.ErrNotFound, what, id)
}

// ---------- Ski (A2) ----------

func (s *MemoryStore) ListSkis() []domain.Ski {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]domain.Ski, 0, len(s.skis))
	for _, k := range s.skis {
		out = append(out, k)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

func (s *MemoryStore) GetSki(id int) (domain.Ski, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	k, ok := s.skis[id]
	if !ok {
		return domain.Ski{}, notFound("Ski", id)
	}
	return k, nil
}

func (s *MemoryStore) CreateSki(k domain.Ski) (domain.Ski, error) {
	k.Normalize()
	if err := k.Validate(); err != nil {
		return domain.Ski{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	k.ID = s.nextSki
	s.nextSki++
	s.skis[k.ID] = k
	return k, nil
}

func (s *MemoryStore) UpdateSki(k domain.Ski) (domain.Ski, error) {
	k.Normalize()
	if err := k.Validate(); err != nil {
		return domain.Ski{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.skis[k.ID]; !ok {
		return domain.Ski{}, notFound("Ski", k.ID)
	}
	s.skis[k.ID] = k
	return k, nil
}

func (s *MemoryStore) DeleteSki(id int) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.skis[id]; !ok {
		return notFound("Ski", id)
	}
	for _, r := range s.rentals {
		if r.SkiID == id && r.IsOpen() {
			return fmt.Errorf("%w: Ski hat noch eine offene Ausleihe (#%d)", domain.ErrConflict, r.ID)
		}
	}
	delete(s.skis, id)
	return nil
}

// AvailableSkis liefert alle Ski, die im Zeitraum [start, end) frei sind (A5).
func (s *MemoryStore) AvailableSkis(start, end, now time.Time) []domain.Ski {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := []domain.Ski{}
	for _, k := range s.skis {
		if !s.skiBlockedLocked(k.ID, start, end, now) {
			out = append(out, k)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

func (s *MemoryStore) skiBlockedLocked(skiID int, start, end, now time.Time) bool {
	for _, r := range s.rentals {
		if r.SkiID == skiID && r.Blocks(start, end, now) {
			return true
		}
	}
	return false
}

// ---------- Personen (A3, A4) ----------

func (s *MemoryStore) ListPersons() []domain.Person {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]domain.Person, 0, len(s.persons))
	for _, p := range s.persons {
		out = append(out, p)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].LastName != out[j].LastName {
			return out[i].LastName < out[j].LastName
		}
		return out[i].FirstName < out[j].FirstName
	})
	return out
}

func (s *MemoryStore) GetPerson(id int) (domain.Person, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	p, ok := s.persons[id]
	if !ok {
		return domain.Person{}, notFound("Person", id)
	}
	return p, nil
}

func (s *MemoryStore) duplicateDocLocked(p domain.Person) error {
	for _, o := range s.persons {
		if o.ID != p.ID && o.SameDocument(p) {
			return fmt.Errorf("%w: Ausweis %s ist bereits bei %s hinterlegt", domain.ErrConflict, p.IDDocNumber, o.FullName())
		}
	}
	return nil
}

func (s *MemoryStore) CreatePerson(p domain.Person, now time.Time) (domain.Person, error) {
	p.Normalize()
	if err := p.Validate(); err != nil {
		return domain.Person{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	p.ID = 0
	if err := s.duplicateDocLocked(p); err != nil {
		return domain.Person{}, err
	}
	p.ID = s.nextPerson
	s.nextPerson++
	p.CreatedAt = now
	p.Document = nil // Upload nur über SetDocument
	s.persons[p.ID] = p
	return p, nil
}

// UpdatePerson ändert Stammdaten; Ausweis-Scan und Anlagedatum bleiben erhalten.
func (s *MemoryStore) UpdatePerson(p domain.Person) (domain.Person, error) {
	p.Normalize()
	if err := p.Validate(); err != nil {
		return domain.Person{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	old, ok := s.persons[p.ID]
	if !ok {
		return domain.Person{}, notFound("Person", p.ID)
	}
	if err := s.duplicateDocLocked(p); err != nil {
		return domain.Person{}, err
	}
	p.Document = old.Document
	p.CreatedAt = old.CreatedAt
	s.persons[p.ID] = p
	return p, nil
}

func (s *MemoryStore) DeletePerson(id int) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.persons[id]; !ok {
		return notFound("Person", id)
	}
	for _, r := range s.rentals {
		if r.PersonID == id && r.IsOpen() {
			return fmt.Errorf("%w: Person hat noch eine offene Ausleihe (#%d)", domain.ErrConflict, r.ID)
		}
	}
	delete(s.persons, id)
	return nil
}

// SetDocument speichert (oder ersetzt mit doc == nil: entfernt) den Ausweis-Scan (A4).
func (s *MemoryStore) SetDocument(personID int, doc *domain.Document) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	p, ok := s.persons[personID]
	if !ok {
		return notFound("Person", personID)
	}
	p.Document = doc
	s.persons[personID] = p
	return nil
}

// ---------- Ausleihen (A5, A6) ----------

func (s *MemoryStore) ListRentals() []domain.Rental {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]domain.Rental, 0, len(s.rentals))
	for _, r := range s.rentals {
		out = append(out, r)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Start.After(out[j].Start) })
	return out
}

func (s *MemoryStore) GetRental(id int) (domain.Rental, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	r, ok := s.rentals[id]
	if !ok {
		return domain.Rental{}, notFound("Ausleihe", id)
	}
	return r, nil
}

// CreateRental legt eine Ausleihe an. Existenz-, Tarif- und Überschneidungsprüfung
// laufen unter einem Lock, damit parallele Buchungen keinen Ski doppelt vergeben (A5).
func (s *MemoryStore) CreateRental(skiID, personID int, start time.Time, hours int, now time.Time) (domain.Rental, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	ski, ok := s.skis[skiID]
	if !ok {
		return domain.Rental{}, notFound("Ski", skiID)
	}
	if _, ok := s.persons[personID]; !ok {
		return domain.Rental{}, notFound("Person", personID)
	}
	r, err := domain.NewRental(skiID, personID, start, hours, s.tariffs[ski.Type], now)
	if err != nil {
		return domain.Rental{}, err
	}
	if s.skiBlockedLocked(skiID, r.Start, r.PlannedEnd(), now) {
		return domain.Rental{}, fmt.Errorf("%w: %s ist im gewählten Zeitraum bereits belegt", domain.ErrConflict, ski.Label())
	}
	r.ID = s.nextRental
	s.nextRental++
	s.rentals[r.ID] = r
	return r, nil
}

// ReturnRental bucht die Rückgabe (A6, B5).
func (s *MemoryStore) ReturnRental(id int, now time.Time) (domain.Rental, error) {
	return s.changeRental(id, func(r *domain.Rental) error { return r.Return(now) })
}

// CancelRental storniert eine Reservierung (A6).
func (s *MemoryStore) CancelRental(id int, now time.Time) (domain.Rental, error) {
	return s.changeRental(id, func(r *domain.Rental) error { return r.Cancel(now) })
}

func (s *MemoryStore) changeRental(id int, change func(*domain.Rental) error) (domain.Rental, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	r, ok := s.rentals[id]
	if !ok {
		return domain.Rental{}, notFound("Ausleihe", id)
	}
	if err := change(&r); err != nil {
		return domain.Rental{}, err
	}
	s.rentals[id] = r
	return r, nil
}

// ---------- Tarife (B2, B6) ----------

func (s *MemoryStore) ListTariffs() []domain.Tariff {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]domain.Tariff, 0, len(domain.SkiTypes))
	for _, t := range domain.SkiTypes {
		out = append(out, s.tariffs[t])
	}
	return out
}

func (s *MemoryStore) GetTariff(t domain.SkiType) (domain.Tariff, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	tr, ok := s.tariffs[t]
	if !ok {
		return domain.Tariff{}, fmt.Errorf("%w: Tarif %q", domain.ErrNotFound, t)
	}
	return tr, nil
}

// UpdateTariff ändert einen Tarif. Bestehende Ausleihen behalten ihren Snapshot (B6).
func (s *MemoryStore) UpdateTariff(t domain.Tariff) (domain.Tariff, error) {
	if err := t.Validate(); err != nil {
		return domain.Tariff{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.tariffs[t.SkiType] = t
	return t, nil
}
