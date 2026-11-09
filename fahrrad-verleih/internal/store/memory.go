// Package store hält alle Daten thread-safe im Speicher.
// Konfliktprüfungen (Doppelbuchung, Löschschutz) laufen atomar unter dem Lock.
package store

import (
	"fmt"
	"sort"
	"sync"
	"time"

	"fahrradverleih/internal/domain"
)

// MemoryStore ist der In-Memory-Speicher. Er gibt immer Kopien heraus.
type MemoryStore struct {
	reg        *domain.Registry
	mu         sync.RWMutex
	items      map[int]domain.Item
	persons    map[int]domain.Person
	rentals    map[int]domain.Rental
	tariffs    map[string]domain.Tariff
	nextItem   int
	nextPerson int
	nextRental int
}

// NewMemoryStore legt einen leeren Store mit den Starttarifen aller Plugins an.
func NewMemoryStore(reg *domain.Registry) *MemoryStore {
	return &MemoryStore{
		reg:        reg,
		items:      map[int]domain.Item{},
		persons:    map[int]domain.Person{},
		rentals:    map[int]domain.Rental{},
		tariffs:    reg.DefaultTariffs(),
		nextItem:   1,
		nextPerson: 1,
		nextRental: 1,
	}
}

func (s *MemoryStore) Registry() *domain.Registry { return s.reg }

func notFound(what string, id int) error {
	return fmt.Errorf("%w: %s %d", domain.ErrNotFound, what, id)
}

func copyItem(it domain.Item) domain.Item {
	attrs := make(map[string]string, len(it.Attributes))
	for k, v := range it.Attributes {
		attrs[k] = v
	}
	it.Attributes = attrs
	return it
}

// ---------- Items (K2) ----------

func (s *MemoryStore) ListItems() []domain.Item {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]domain.Item, 0, len(s.items))
	for _, it := range s.items {
		out = append(out, copyItem(it))
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

func (s *MemoryStore) GetItem(id int) (domain.Item, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	it, ok := s.items[id]
	if !ok {
		return domain.Item{}, notFound("Artikel", id)
	}
	return copyItem(it), nil
}

func (s *MemoryStore) CreateItem(it domain.Item) (domain.Item, error) {
	it.Normalize()
	if err := s.reg.ValidateItem(it); err != nil {
		return domain.Item{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	it.ID = s.nextItem
	s.nextItem++
	s.items[it.ID] = copyItem(it)
	return it, nil
}

func (s *MemoryStore) UpdateItem(it domain.Item) (domain.Item, error) {
	it.Normalize()
	if err := s.reg.ValidateItem(it); err != nil {
		return domain.Item{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.items[it.ID]; !ok {
		return domain.Item{}, notFound("Artikel", it.ID)
	}
	s.items[it.ID] = copyItem(it)
	return it, nil
}

func (s *MemoryStore) DeleteItem(id int) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.items[id]; !ok {
		return notFound("Artikel", id)
	}
	for _, r := range s.rentals {
		if r.ItemID == id && r.IsOpen() {
			return fmt.Errorf("%w: Artikel hat noch eine offene Ausleihe (#%d)", domain.ErrConflict, r.ID)
		}
	}
	delete(s.items, id)
	return nil
}

// AvailableItems liefert alle Items, die im Zeitraum [start, end) frei sind (K5).
func (s *MemoryStore) AvailableItems(start, end, now time.Time) []domain.Item {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := []domain.Item{}
	for _, it := range s.items {
		if !s.itemBlockedLocked(it.ID, start, end, now) {
			out = append(out, copyItem(it))
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

func (s *MemoryStore) itemBlockedLocked(itemID int, start, end, now time.Time) bool {
	for _, r := range s.rentals {
		if r.ItemID == itemID && r.Blocks(start, end, now) {
			return true
		}
	}
	return false
}

// ---------- Personen (K3, K4) ----------

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

// SetDocument speichert den Ausweis-Scan; doc == nil entfernt ihn (K4).
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

// ---------- Ausleihen (K5, K6, C3) ----------

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

// CreateRental legt eine Ausleihe an. Existenz-, Zubehör-, Tarif- und Überschneidungsprüfung
// laufen unter einem Lock, damit parallele Buchungen kein Item doppelt vergeben (K5).
func (s *MemoryStore) CreateRental(itemID, personID int, start time.Time, hours int, extraKeys []string, now time.Time) (domain.Rental, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	it, ok := s.items[itemID]
	if !ok {
		return domain.Rental{}, notFound("Artikel", itemID)
	}
	if _, ok := s.persons[personID]; !ok {
		return domain.Rental{}, notFound("Person", personID)
	}
	extras, err := s.reg.ResolveExtras(it, extraKeys)
	if err != nil {
		return domain.Rental{}, err
	}
	r, err := domain.NewRental(itemID, personID, start, hours, s.tariffs[domain.TariffKey(it.Category, it.Kind)], extras, now)
	if err != nil {
		return domain.Rental{}, err
	}
	if s.itemBlockedLocked(itemID, r.Start, r.PlannedEnd(), now) {
		return domain.Rental{}, fmt.Errorf("%w: %s ist im gewählten Zeitraum bereits belegt", domain.ErrConflict, s.reg.Label(it))
	}
	r.ID = s.nextRental
	s.nextRental++
	s.rentals[r.ID] = r
	return r, nil
}

func (s *MemoryStore) ReturnRental(id int, now time.Time) (domain.Rental, error) {
	return s.changeRental(id, func(r *domain.Rental) error { return r.Return(now) })
}

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

// ---------- Tarife (K8, K10) ----------

// ListTariffs in Plugin-/Typ-Reihenfolge.
func (s *MemoryStore) ListTariffs() []domain.Tariff {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := []domain.Tariff{}
	for _, p := range s.reg.All() {
		for _, k := range p.Kinds() {
			out = append(out, s.tariffs[domain.TariffKey(p.Category(), k.Key)])
		}
	}
	return out
}

func (s *MemoryStore) GetTariff(category, kind string) (domain.Tariff, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	t, ok := s.tariffs[domain.TariffKey(category, kind)]
	if !ok {
		return domain.Tariff{}, fmt.Errorf("%w: Tarif %s/%s", domain.ErrNotFound, category, kind)
	}
	return t, nil
}

// UpdateTariff ändert einen Tarif; bestehende Ausleihen behalten ihren Snapshot (K10).
func (s *MemoryStore) UpdateTariff(t domain.Tariff) (domain.Tariff, error) {
	if _, ok := s.reg.Kind(t.Category, t.Kind); !ok {
		return domain.Tariff{}, fmt.Errorf("%w: Typ %s/%s", domain.ErrNotFound, t.Category, t.Kind)
	}
	if err := t.Validate(); err != nil {
		return domain.Tariff{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.tariffs[t.Key()] = t
	return t, nil
}
