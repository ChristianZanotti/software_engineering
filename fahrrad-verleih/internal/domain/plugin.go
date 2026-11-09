package domain

import (
	"fmt"
	"slices"
	"strconv"
)

// Plugin beschreibt eine Kategorie verleihbarer Items (C2).
// Der Kern kennt nur dieses Interface – fachliches Wissen (z. B. über Fahrräder) steckt im Plugin.
type Plugin interface {
	Category() string    // technischer Schlüssel, z. B. "bike"
	Name() string        // Anzeigename Singular, z. B. "Fahrrad"
	NamePlural() string  // Anzeigename Plural, z. B. "Fahrräder"
	Kinds() []Kind       // Typen inkl. Starttarif
	Fields() []Field     // Zusatzattribute
	Extras() []Extra     // buchbares Zubehör
	Validate(Item) error // fachliche Regeln über mehrere Felder (C4); generische Prüfung macht die Registry
	Label(Item) string   // kurze Anzeige, z. B. "Cube Kathmandu (M)"
}

// Kind ist ein Typ innerhalb einer Kategorie mit Starttarif.
type Kind struct {
	Key         string `json:"key"`
	Label       string `json:"label"`
	HourlyCents int64  `json:"hourlyCents"`
	DailyCents  int64  `json:"dailyCents"`
}

// FieldType bestimmt Eingabeelement und Prüfung eines Attributs.
type FieldType string

const (
	FieldText   FieldType = "text"
	FieldNumber FieldType = "number"
	FieldSelect FieldType = "select"
)

// Field ist ein plugin-spezifisches Attribut. Das Frontend baut daraus Formulare (C2).
type Field struct {
	Key       string    `json:"key"`
	Label     string    `json:"label"`
	Type      FieldType `json:"type"`
	Required  bool      `json:"required"`
	Options   []string  `json:"options,omitempty"`
	Min       int       `json:"min,omitempty"`
	Max       int       `json:"max,omitempty"`
	Unit      string    `json:"unit,omitempty"`
	OnlyKinds []string  `json:"onlyKinds,omitempty"` // leer = gilt für alle Typen
	InTable   bool      `json:"inTable"`             // als Tabellenspalte anzeigen
}

// AppliesTo meldet, ob das Feld für den Typ relevant ist.
func (f Field) AppliesTo(kind string) bool {
	return len(f.OnlyKinds) == 0 || slices.Contains(f.OnlyKinds, kind)
}

// Extra ist buchbares Zubehör mit Pauschalpreis pro Ausleihe (C3).
type Extra struct {
	Key        string   `json:"key"`
	Label      string   `json:"label"`
	PriceCents int64    `json:"priceCents"`
	OnlyKinds  []string `json:"onlyKinds,omitempty"`
}

func (e Extra) AppliesTo(kind string) bool {
	return len(e.OnlyKinds) == 0 || slices.Contains(e.OnlyKinds, kind)
}

// Registry hält alle registrierten Plugins in Registrierungsreihenfolge.
type Registry struct {
	plugins []Plugin
}

// NewRegistry registriert Plugins. Doppelte Kategorien sind ein Programmierfehler.
func NewRegistry(plugins ...Plugin) *Registry {
	r := &Registry{}
	for _, p := range plugins {
		if _, ok := r.Get(p.Category()); ok {
			panic(fmt.Sprintf("Plugin %q doppelt registriert", p.Category()))
		}
		r.plugins = append(r.plugins, p)
	}
	return r
}

func (r *Registry) All() []Plugin { return r.plugins }

func (r *Registry) Get(category string) (Plugin, bool) {
	for _, p := range r.plugins {
		if p.Category() == category {
			return p, true
		}
	}
	return nil, false
}

// Kind sucht einen Typ innerhalb einer Kategorie.
func (r *Registry) Kind(category, kind string) (Kind, bool) {
	p, ok := r.Get(category)
	if !ok {
		return Kind{}, false
	}
	for _, k := range p.Kinds() {
		if k.Key == kind {
			return k, true
		}
	}
	return Kind{}, false
}

// Label liefert die Plugin-Anzeige eines Items.
func (r *Registry) Label(it Item) string {
	if p, ok := r.Get(it.Category); ok {
		return p.Label(it)
	}
	return it.Brand + " " + it.Model
}

// ValidateItem prüft Basisfelder, Kategorie/Typ, alle Plugin-Felder generisch und zuletzt die Plugin-Regeln (C2, C4).
func (r *Registry) ValidateItem(it Item) error {
	v := validator{problems: it.validateBase()}
	p, ok := r.Get(it.Category)
	if !ok {
		v.check(false, "Kategorie ist unbekannt")
		return v.err()
	}
	_, kindOK := r.Kind(it.Category, it.Kind)
	v.check(kindOK, "Typ ist ungültig")

	known := map[string]bool{}
	for _, f := range p.Fields() {
		known[f.Key] = true
		val, has := it.Attributes[f.Key]
		if !f.AppliesTo(it.Kind) {
			v.check(!has, fmt.Sprintf("%s ist für diesen Typ nicht vorgesehen", f.Label))
			continue
		}
		if !has {
			v.check(!f.Required, fmt.Sprintf("%s fehlt", f.Label))
			continue
		}
		switch f.Type {
		case FieldNumber:
			n, err := strconv.Atoi(val)
			v.check(err == nil && n >= f.Min && n <= f.Max, fmt.Sprintf("%s muss zwischen %d und %d liegen", f.Label, f.Min, f.Max))
		case FieldSelect:
			v.check(slices.Contains(f.Options, val), fmt.Sprintf("%s: ungültiger Wert %q", f.Label, val))
		}
	}
	for k := range it.Attributes {
		v.check(known[k], fmt.Sprintf("Unbekanntes Feld %q", k))
	}
	if err := v.err(); err != nil {
		return err
	}
	return p.Validate(it)
}

// ResolveExtras übersetzt Zubehör-Schlüssel in Extras (Preis-Snapshot) und prüft, ob sie zum Typ passen (C3).
func (r *Registry) ResolveExtras(it Item, keys []string) ([]Extra, error) {
	p, ok := r.Get(it.Category)
	if !ok {
		return nil, fmt.Errorf("%w: Kategorie %q", ErrNotFound, it.Category)
	}
	var v validator
	out := []Extra{}
	seen := map[string]bool{}
	for _, key := range keys {
		if seen[key] {
			continue
		}
		seen[key] = true
		idx := slices.IndexFunc(p.Extras(), func(e Extra) bool { return e.Key == key })
		if idx < 0 {
			v.check(false, fmt.Sprintf("Unbekanntes Zubehör %q", key))
			continue
		}
		e := p.Extras()[idx]
		v.check(e.AppliesTo(it.Kind), fmt.Sprintf("%s ist für diesen Typ nicht buchbar", e.Label))
		out = append(out, e)
	}
	if err := v.err(); err != nil {
		return nil, err
	}
	return out, nil
}

// DefaultTariffs sammelt die Starttarife aller Plugins.
func (r *Registry) DefaultTariffs() map[string]Tariff {
	out := map[string]Tariff{}
	for _, p := range r.plugins {
		for _, k := range p.Kinds() {
			t := Tariff{Category: p.Category(), Kind: k.Key, HourlyCents: k.HourlyCents, DailyCents: k.DailyCents}
			out[t.Key()] = t
		}
	}
	return out
}
