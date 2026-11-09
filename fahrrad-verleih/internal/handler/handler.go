// Package handler übersetzt HTTP ⇄ Domain/Store. Keine Business-Logik und kein Plugin-Wissen hier.
package handler

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"fahrradverleih/internal/domain"
	"fahrradverleih/internal/store"
)

// API bündelt alle Endpunkte. now ist austauschbar für Tests.
type API struct {
	appName string
	store   *store.MemoryStore
	now     func() time.Time
}

func New(appName string, s *store.MemoryStore) *API {
	return &API{appName: appName, store: s, now: time.Now}
}

// Register hängt alle Routen (README, Abschnitt REST-API) an den Mux.
func (a *API) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/plugins", a.plugins)
	mux.HandleFunc("GET /api/dashboard", a.dashboard)

	mux.HandleFunc("GET /api/items", a.listItems)
	mux.HandleFunc("POST /api/items", a.createItem)
	mux.HandleFunc("GET /api/items/available", a.availableItems)
	mux.HandleFunc("GET /api/items/{id}", a.getItem)
	mux.HandleFunc("PUT /api/items/{id}", a.updateItem)
	mux.HandleFunc("DELETE /api/items/{id}", a.deleteItem)

	mux.HandleFunc("GET /api/persons", a.listPersons)
	mux.HandleFunc("POST /api/persons", a.createPerson)
	mux.HandleFunc("GET /api/persons/{id}", a.getPerson)
	mux.HandleFunc("PUT /api/persons/{id}", a.updatePerson)
	mux.HandleFunc("DELETE /api/persons/{id}", a.deletePerson)
	mux.HandleFunc("GET /api/persons/{id}/document", a.getDocument)
	mux.HandleFunc("POST /api/persons/{id}/document", a.uploadDocument)
	mux.HandleFunc("DELETE /api/persons/{id}/document", a.deleteDocument)

	mux.HandleFunc("GET /api/rentals", a.listRentals)
	mux.HandleFunc("POST /api/rentals", a.createRental)
	mux.HandleFunc("GET /api/rentals/quote", a.quote)
	mux.HandleFunc("GET /api/rentals/{id}/return-quote", a.returnQuote)
	mux.HandleFunc("POST /api/rentals/{id}/return", a.returnRental)
	mux.HandleFunc("POST /api/rentals/{id}/cancel", a.cancelRental)

	mux.HandleFunc("GET /api/prices", a.listPrices)
	mux.HandleFunc("PUT /api/prices/{category}/{kind}", a.updatePrice)
}

// ---------- Hilfsfunktionen ----------

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("JSON schreiben: %v", err)
	}
}

// writeError bildet Domain-Fehler auf Statuscodes ab (siehe CLAUDE.md).
func writeError(w http.ResponseWriter, err error) {
	status := http.StatusInternalServerError
	switch {
	case errors.Is(err, domain.ErrValidation):
		status = http.StatusBadRequest
	case errors.Is(err, domain.ErrNotFound):
		status = http.StatusNotFound
	case errors.Is(err, domain.ErrConflict):
		status = http.StatusConflict
	}
	msg := err.Error()
	var ve *domain.ValidationError
	if errors.As(err, &ve) {
		msg = ve.Error()
	}
	if status == http.StatusInternalServerError {
		log.Printf("interner Fehler: %v", err)
		msg = "Interner Fehler"
	}
	writeJSON(w, status, map[string]string{"error": msg})
}

func badRequest(msg string) error {
	return &domain.ValidationError{Problems: []string{msg}}
}

func decode(r *http.Request, v any) error {
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(v); err != nil {
		return badRequest("Ungültiges JSON: " + err.Error())
	}
	return nil
}

func pathID(r *http.Request) (int, error) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil || id <= 0 {
		return 0, badRequest("Ungültige ID")
	}
	return id, nil
}

func queryInt(r *http.Request, key string) (int, error) {
	n, err := strconv.Atoi(r.URL.Query().Get(key))
	if err != nil {
		return 0, badRequest(fmt.Sprintf("Parameter %q fehlt oder ist keine Zahl", key))
	}
	return n, nil
}

// ---------- Plugin-Metadaten (C2) ----------

// PluginInfo ist die JSON-Form eines Plugins für das generische Frontend.
type PluginInfo struct {
	Category   string         `json:"category"`
	Name       string         `json:"name"`
	NamePlural string         `json:"namePlural"`
	Kinds      []domain.Kind  `json:"kinds"`
	Fields     []domain.Field `json:"fields"`
	Extras     []domain.Extra `json:"extras"`
}

func (a *API) plugins(w http.ResponseWriter, r *http.Request) {
	infos := []PluginInfo{}
	for _, p := range a.store.Registry().All() {
		infos = append(infos, PluginInfo{p.Category(), p.Name(), p.NamePlural(), p.Kinds(), p.Fields(), p.Extras()})
	}
	writeJSON(w, http.StatusOK, map[string]any{"appName": a.appName, "plugins": infos})
}

// ---------- View-Structs ----------

// ItemView ergänzt das Item um Label, Status und nächste Reservierung.
type ItemView struct {
	domain.Item
	Label           string     `json:"label"`
	Status          string     `json:"status"` // verfuegbar | ausgeliehen | ueberfaellig
	NextReservation *time.Time `json:"nextReservation,omitempty"`
}

type PersonView struct {
	domain.Person
	OpenRentals int `json:"openRentals"`
}

type RentalView struct {
	domain.Rental
	Status     domain.RentalStatus `json:"status"`
	PlannedEnd time.Time           `json:"plannedEnd"`
	ItemLabel  string              `json:"itemLabel"`
	PersonName string              `json:"personName"`
}

func (a *API) itemViews(items []domain.Item, now time.Time) []ItemView {
	rentals := a.store.ListRentals()
	reg := a.store.Registry()
	out := make([]ItemView, 0, len(items))
	for _, it := range items {
		v := ItemView{Item: it, Label: reg.Label(it), Status: "verfuegbar"}
		for _, r := range rentals {
			if r.ItemID != it.ID {
				continue
			}
			switch r.Status(now) {
			case domain.StatusActive:
				v.Status = "ausgeliehen"
			case domain.StatusOverdue:
				v.Status = "ueberfaellig"
			case domain.StatusReserved:
				if v.NextReservation == nil || r.Start.Before(*v.NextReservation) {
					t := r.Start
					v.NextReservation = &t
				}
			}
		}
		out = append(out, v)
	}
	return out
}

func (a *API) rentalViews(rentals []domain.Rental, now time.Time) []RentalView {
	reg := a.store.Registry()
	items := map[int]domain.Item{}
	for _, it := range a.store.ListItems() {
		items[it.ID] = it
	}
	persons := map[int]domain.Person{}
	for _, p := range a.store.ListPersons() {
		persons[p.ID] = p
	}
	out := make([]RentalView, 0, len(rentals))
	for _, r := range rentals {
		v := RentalView{Rental: r, Status: r.Status(now), PlannedEnd: r.PlannedEnd()}
		if it, ok := items[r.ItemID]; ok {
			v.ItemLabel = reg.Label(it)
		} else {
			v.ItemLabel = fmt.Sprintf("Artikel #%d (gelöscht)", r.ItemID)
		}
		if p, ok := persons[r.PersonID]; ok {
			v.PersonName = p.FullName()
		} else {
			v.PersonName = fmt.Sprintf("Person #%d (gelöscht)", r.PersonID)
		}
		out = append(out, v)
	}
	return out
}

// ---------- Dashboard (K1, C5) ----------

func (a *API) dashboard(w http.ResponseWriter, r *http.Request) {
	now := a.now()
	d := domain.BuildDashboard(a.store.Registry(), a.store.ListItems(), a.store.ListPersons(), a.store.ListRentals(), now)
	writeJSON(w, http.StatusOK, struct {
		domain.Dashboard
		Overdue              []RentalView `json:"overdue"`
		DueToday             []RentalView `json:"dueToday"`
		UpcomingReservations []RentalView `json:"upcomingReservations"`
	}{d, a.rentalViews(d.Overdue, now), a.rentalViews(d.DueToday, now), a.rentalViews(d.UpcomingReservations, now)})
}

// ---------- Items (K2, K5) ----------

func (a *API) listItems(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, a.itemViews(a.store.ListItems(), a.now()))
}

func (a *API) getItem(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		writeError(w, err)
		return
	}
	it, err := a.store.GetItem(id)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, a.itemViews([]domain.Item{it}, a.now())[0])
}

func (a *API) createItem(w http.ResponseWriter, r *http.Request) {
	var it domain.Item
	if err := decode(r, &it); err != nil {
		writeError(w, err)
		return
	}
	created, err := a.store.CreateItem(it)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, created)
}

func (a *API) updateItem(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		writeError(w, err)
		return
	}
	var it domain.Item
	if err := decode(r, &it); err != nil {
		writeError(w, err)
		return
	}
	it.ID = id
	updated, err := a.store.UpdateItem(it)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, updated)
}

func (a *API) deleteItem(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		writeError(w, err)
		return
	}
	if err := a.store.DeleteItem(id); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (a *API) availableItems(w http.ResponseWriter, r *http.Request) {
	start, err := time.Parse(time.RFC3339, r.URL.Query().Get("start"))
	if err != nil {
		writeError(w, badRequest("Parameter start muss RFC 3339 sein"))
		return
	}
	hours, err := queryInt(r, "hours")
	if err != nil {
		writeError(w, err)
		return
	}
	if hours < 1 {
		hours = 1
	}
	now := a.now()
	end := start.Add(time.Duration(hours) * time.Hour)
	writeJSON(w, http.StatusOK, a.itemViews(a.store.AvailableItems(start, end, now), now))
}

// ---------- Personen (K3, K4) ----------

func (a *API) personViews(persons []domain.Person) []PersonView {
	open := map[int]int{}
	for _, r := range a.store.ListRentals() {
		if r.IsOpen() {
			open[r.PersonID]++
		}
	}
	out := make([]PersonView, 0, len(persons))
	for _, p := range persons {
		out = append(out, PersonView{Person: p, OpenRentals: open[p.ID]})
	}
	return out
}

func (a *API) listPersons(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, a.personViews(a.store.ListPersons()))
}

func (a *API) getPerson(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		writeError(w, err)
		return
	}
	p, err := a.store.GetPerson(id)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, a.personViews([]domain.Person{p})[0])
}

func (a *API) createPerson(w http.ResponseWriter, r *http.Request) {
	var p domain.Person
	if err := decode(r, &p); err != nil {
		writeError(w, err)
		return
	}
	created, err := a.store.CreatePerson(p, a.now())
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, created)
}

func (a *API) updatePerson(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		writeError(w, err)
		return
	}
	var p domain.Person
	if err := decode(r, &p); err != nil {
		writeError(w, err)
		return
	}
	p.ID = id
	updated, err := a.store.UpdatePerson(p)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, updated)
}

func (a *API) deletePerson(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		writeError(w, err)
		return
	}
	if err := a.store.DeletePerson(id); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (a *API) uploadDocument(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		writeError(w, err)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, domain.MaxDocumentBytes+1<<20)
	file, header, err := r.FormFile("file")
	if err != nil {
		writeError(w, badRequest("Keine Datei erhalten oder Datei größer als 5 MB"))
		return
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, domain.MaxDocumentBytes+1))
	if err != nil {
		writeError(w, badRequest("Datei konnte nicht gelesen werden"))
		return
	}
	doc, err := domain.NewDocument(header.Filename, data, a.now())
	if err != nil {
		writeError(w, err)
		return
	}
	if err := a.store.SetDocument(id, &doc); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, doc)
}

func (a *API) getDocument(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		writeError(w, err)
		return
	}
	p, err := a.store.GetPerson(id)
	if err != nil {
		writeError(w, err)
		return
	}
	if p.Document == nil {
		writeError(w, fmt.Errorf("%w: kein Ausweis-Scan hinterlegt", domain.ErrNotFound))
		return
	}
	w.Header().Set("Content-Type", p.Document.ContentType)
	w.Header().Set("Content-Disposition", fmt.Sprintf("inline; filename=%q", p.Document.FileName))
	w.Header().Set("X-Content-Type-Options", "nosniff")
	_, _ = w.Write(p.Document.Data)
}

func (a *API) deleteDocument(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		writeError(w, err)
		return
	}
	if err := a.store.SetDocument(id, nil); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ---------- Ausleihen (K5, K6, C3) ----------

func (a *API) listRentals(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, a.rentalViews(a.store.ListRentals(), a.now()))
}

type createRentalRequest struct {
	ItemID        int       `json:"itemId"`
	PersonID      int       `json:"personId"`
	Start         time.Time `json:"start"`
	DurationHours int       `json:"durationHours"`
	Extras        []string  `json:"extras"`
}

func (a *API) createRental(w http.ResponseWriter, r *http.Request) {
	var req createRentalRequest
	if err := decode(r, &req); err != nil {
		writeError(w, err)
		return
	}
	now := a.now()
	rental, err := a.store.CreateRental(req.ItemID, req.PersonID, req.Start, req.DurationHours, req.Extras, now)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, a.rentalViews([]domain.Rental{rental}, now)[0])
}

// returnQuote zeigt den Endpreis vor der Rückgabe (K6, K9).
func (a *API) returnQuote(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		writeError(w, err)
		return
	}
	rental, err := a.store.GetRental(id)
	if err != nil {
		writeError(w, err)
		return
	}
	now := a.now()
	writeJSON(w, http.StatusOK, struct {
		Rental RentalView   `json:"rental"`
		Quote  domain.Quote `json:"quote"`
	}{a.rentalViews([]domain.Rental{rental}, now)[0], rental.ReturnQuote(now)})
}

func (a *API) returnRental(w http.ResponseWriter, r *http.Request) {
	a.changeRental(w, r, a.store.ReturnRental)
}

func (a *API) cancelRental(w http.ResponseWriter, r *http.Request) {
	a.changeRental(w, r, a.store.CancelRental)
}

func (a *API) changeRental(w http.ResponseWriter, r *http.Request, change func(int, time.Time) (domain.Rental, error)) {
	id, err := pathID(r)
	if err != nil {
		writeError(w, err)
		return
	}
	now := a.now()
	rental, err := change(id, now)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, a.rentalViews([]domain.Rental{rental}, now)[0])
}

// quote liefert die Preisvorschau inkl. Zubehör (C6) – für ein Item oder Kategorie+Typ.
func (a *API) quote(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	hours, err := queryInt(r, "hours")
	if err != nil {
		writeError(w, err)
		return
	}
	if hours < 1 || hours > domain.MaxRentalHours {
		writeError(w, badRequest("Verleihdauer muss zwischen 1 Stunde und 30 Tagen liegen"))
		return
	}
	it := domain.Item{Category: q.Get("category"), Kind: q.Get("kind")}
	if q.Has("itemId") {
		id, err := queryInt(r, "itemId")
		if err != nil {
			writeError(w, err)
			return
		}
		if it, err = a.store.GetItem(id); err != nil {
			writeError(w, err)
			return
		}
	}
	tariff, err := a.store.GetTariff(it.Category, it.Kind)
	if err != nil {
		writeError(w, err)
		return
	}
	var keys []string
	if s := q.Get("extras"); s != "" {
		keys = strings.Split(s, ",")
	}
	extras, err := a.store.Registry().ResolveExtras(it, keys)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, tariff.Quote(hours, extras))
}

// ---------- Tarife (K8, K10) ----------

func (a *API) listPrices(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, a.store.ListTariffs())
}

func (a *API) updatePrice(w http.ResponseWriter, r *http.Request) {
	var t domain.Tariff
	if err := decode(r, &t); err != nil {
		writeError(w, err)
		return
	}
	t.Category, t.Kind = r.PathValue("category"), r.PathValue("kind")
	updated, err := a.store.UpdateTariff(t)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, updated)
}
