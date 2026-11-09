package domain

import (
	"errors"
	"testing"
	"time"
)

var t0 = time.Date(2026, 6, 1, 9, 0, 0, 0, time.UTC)

// skiPlugin ist ein Test-Plugin: Es beweist, dass ein neuer Artikeltyp ohne Änderung am Kern funktioniert (C2).
type skiPlugin struct{}

func (skiPlugin) Category() string   { return "ski" }
func (skiPlugin) Name() string       { return "Ski" }
func (skiPlugin) NamePlural() string { return "Ski" }
func (skiPlugin) Kinds() []Kind {
	return []Kind{{Key: "alpin", Label: "Alpin", HourlyCents: 800, DailyCents: 3500}}
}
func (skiPlugin) Fields() []Field {
	return []Field{{Key: "lengthCm", Label: "Länge", Type: FieldNumber, Required: true, Min: 80, Max: 220}}
}
func (skiPlugin) Extras() []Extra {
	return []Extra{{Key: "stoecke", Label: "Stöcke", PriceCents: 300}}
}
func (skiPlugin) Validate(Item) error  { return nil }
func (skiPlugin) Label(it Item) string { return it.Brand + " " + it.Model }

func skiItem(length string) Item {
	return Item{Category: "ski", Kind: "alpin", Brand: "Atomic", Model: "X", Condition: ConditionGood, Attributes: map[string]string{"lengthCm": length}}
}

func TestC2_NewPluginWorksWithoutCoreChanges(t *testing.T) {
	reg := NewRegistry(skiPlugin{})
	if err := reg.ValidateItem(skiItem("170")); err != nil {
		t.Fatalf("gültiger Ski abgelehnt: %v", err)
	}
	if err := reg.ValidateItem(skiItem("300")); !errors.Is(err, ErrValidation) {
		t.Errorf("Länge 300 muss generisch abgelehnt werden, err=%v", err)
	}
	tariffs := reg.DefaultTariffs()
	if tariffs["ski/alpin"].DailyCents != 3500 {
		t.Errorf("Starttarif aus Plugin fehlt: %+v", tariffs)
	}
	extras, err := reg.ResolveExtras(skiItem("170"), []string{"stoecke"})
	if err != nil || len(extras) != 1 {
		t.Errorf("Plugin-Zubehör: %v %v", extras, err)
	}
}

func TestC2_GenericFieldValidation(t *testing.T) {
	reg := NewRegistry(skiPlugin{})
	cases := map[string]Item{
		"unbekannte Kategorie": {Category: "boot", Kind: "x", Brand: "a", Model: "b", Condition: ConditionGood},
		"unbekannter Typ":      {Category: "ski", Kind: "x", Brand: "a", Model: "b", Condition: ConditionGood, Attributes: map[string]string{"lengthCm": "170"}},
		"Pflichtfeld fehlt":    {Category: "ski", Kind: "alpin", Brand: "a", Model: "b", Condition: ConditionGood},
		"unbekanntes Feld":     {Category: "ski", Kind: "alpin", Brand: "a", Model: "b", Condition: ConditionGood, Attributes: map[string]string{"lengthCm": "170", "foo": "1"}},
		"Basisfelder fehlen":   {Category: "ski", Kind: "alpin", Attributes: map[string]string{"lengthCm": "170"}},
	}
	for name, it := range cases {
		if err := reg.ValidateItem(it); !errors.Is(err, ErrValidation) {
			t.Errorf("%s: erwartet Validierungsfehler, err=%v", name, err)
		}
	}
}

func TestC2_DuplicatePluginPanics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("doppelte Registrierung muss panic auslösen")
		}
	}()
	NewRegistry(skiPlugin{}, skiPlugin{})
}

func tariff() Tariff {
	return Tariff{Category: "ski", Kind: "alpin", HourlyCents: 800, DailyCents: 3500}
}

// K8: Akzeptanzbeispiele, unverändert aus dem Ski-Verleih.
func TestK8_PriceCalculation(t *testing.T) {
	cases := map[int]int64{1: 800, 3: 2400, 5: 3500, 24: 3500, 26: 5100, 30: 7000}
	for h, want := range cases {
		if got := tariff().Quote(h, nil).TotalCents; got != want {
			t.Errorf("%d h = %d, want %d", h, got, want)
		}
	}
}

// C3: Zubehör pauschal auf geplanten und finalen Preis.
func TestC3_ExtrasAddedFlat(t *testing.T) {
	extras := []Extra{{Key: "helm", PriceCents: 200}, {Key: "schloss", PriceCents: 100}}
	q := tariff().Quote(26, extras)
	if q.RentCents != 5100 || q.ExtrasCents != 300 || q.TotalCents != 5400 {
		t.Errorf("Quote mit Zubehör: %+v", q)
	}
	r, err := NewRental(1, 1, t0, 3, tariff(), extras, t0)
	if err != nil {
		t.Fatal(err)
	}
	if r.PlannedPriceCents != 2400+300 {
		t.Errorf("geplanter Preis: %d", r.PlannedPriceCents)
	}
	if err := r.Return(t0.Add(30 * time.Minute)); err != nil {
		t.Fatal(err)
	}
	if *r.FinalPriceCents != 800+300 {
		t.Errorf("finaler Preis: %d", *r.FinalPriceCents)
	}
}

func TestC3_ExtrasRestrictedByKind(t *testing.T) {
	reg := NewRegistry(skiPlugin{})
	if _, err := reg.ResolveExtras(skiItem("170"), []string{"gibtsnicht"}); !errors.Is(err, ErrValidation) {
		t.Errorf("unbekanntes Zubehör: err=%v", err)
	}
	extras, err := reg.ResolveExtras(skiItem("170"), []string{"stoecke", "stoecke"})
	if err != nil || len(extras) != 1 {
		t.Errorf("Duplikate müssen zusammengefasst werden: %v %v", extras, err)
	}
}

func TestK9_BilledHours(t *testing.T) {
	cases := map[time.Duration]int{0: 1, time.Minute: 1, time.Hour: 1, time.Hour + time.Second: 2, 210 * time.Minute: 4}
	for d, want := range cases {
		if got := BilledHours(d); got != want {
			t.Errorf("BilledHours(%v) = %d, want %d", d, got, want)
		}
	}
}

func TestK5_BlocksAndStatus(t *testing.T) {
	r, _ := NewRental(1, 1, t0, 4, tariff(), nil, t0)
	h := time.Hour
	if r.Blocks(t0.Add(4*h), t0.Add(5*h), t0) {
		t.Error("direkt anschließend darf nicht blockieren")
	}
	if !r.Blocks(t0.Add(3*h), t0.Add(5*h), t0) {
		t.Error("Überlappung muss blockieren")
	}
	if !r.Blocks(t0.Add(100*h), t0.Add(101*h), t0.Add(5*h)) {
		t.Error("überfällig muss alles blockieren")
	}
	if r.Status(t0.Add(-h)) != StatusReserved || r.Status(t0) != StatusActive || r.Status(t0.Add(4*h)) != StatusOverdue {
		t.Error("Statusableitung falsch")
	}
}

func TestK6_ReturnAndCancelRules(t *testing.T) {
	r, _ := NewRental(1, 1, t0, 2, tariff(), nil, t0)
	if err := r.Return(t0.Add(-time.Minute)); !errors.Is(err, ErrConflict) {
		t.Error("Reservierung darf nicht zurückgegeben werden")
	}
	if err := r.Cancel(t0.Add(time.Minute)); !errors.Is(err, ErrConflict) {
		t.Error("aktive Ausleihe darf nicht storniert werden")
	}
}

func TestK5_Validation(t *testing.T) {
	if _, err := NewRental(1, 1, t0, 0, tariff(), nil, t0); !errors.Is(err, ErrValidation) {
		t.Error("Dauer 0")
	}
	if _, err := NewRental(1, 1, t0.Add(-time.Hour), 1, tariff(), nil, t0); !errors.Is(err, ErrValidation) {
		t.Error("Beginn in der Vergangenheit")
	}
}

func TestK4_DocumentContentCheck(t *testing.T) {
	if _, err := NewDocument("x.png", []byte("<html></html>"), t0); !errors.Is(err, ErrValidation) {
		t.Error("HTML als PNG muss abgelehnt werden")
	}
	if _, err := NewDocument("a.pdf", []byte("%PDF-1.4"), t0); err != nil {
		t.Errorf("PDF: %v", err)
	}
}

func TestK1_C5_Dashboard(t *testing.T) {
	reg := NewRegistry(skiPlugin{})
	items := []Item{{ID: 1, Category: "ski", Kind: "alpin"}, {ID: 2, Category: "ski", Kind: "alpin"}}
	active, _ := NewRental(1, 1, t0, 4, tariff(), nil, t0)
	d := BuildDashboard(reg, items, []Person{{ID: 1}}, []Rental{active}, t0.Add(time.Hour))
	if d.TotalItems != 2 || d.RentedItems != 1 || d.AvailableItems != 1 || d.PersonsWithoutDoc != 1 {
		t.Errorf("Kennzahlen: %+v", d)
	}
	if len(d.ByKind) != 1 || d.ByKind[0].Total != 2 || d.ByKind[0].Available != 1 || d.ByKind[0].Label != "Alpin" {
		t.Errorf("Verfügbarkeit nach Typ: %+v", d.ByKind)
	}
}
