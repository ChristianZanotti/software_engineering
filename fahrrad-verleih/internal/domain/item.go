package domain

import "strings"

// Condition beschreibt den Zustand eines Items.
type Condition string

const (
	ConditionNew  Condition = "neu"
	ConditionGood Condition = "gut"
	ConditionUsed Condition = "gebraucht"
)

func (c Condition) Valid() bool {
	return c == ConditionNew || c == ConditionGood || c == ConditionUsed
}

// Item ist der generische verleihbare Gegenstand (RentalItem, C2).
// Kategorie und Typ bestimmen das Plugin bzw. den Tarif, Attribute sind plugin-spezifisch.
type Item struct {
	ID         int               `json:"id"`
	Category   string            `json:"category"`
	Kind       string            `json:"kind"`
	Brand      string            `json:"brand"`
	Model      string            `json:"model"`
	Condition  Condition         `json:"condition"`
	Attributes map[string]string `json:"attributes"`
	Notes      string            `json:"notes"`
}

// Normalize entfernt Leerzeichen und leere Attribute.
func (it *Item) Normalize() {
	it.Brand = strings.TrimSpace(it.Brand)
	it.Model = strings.TrimSpace(it.Model)
	it.Notes = strings.TrimSpace(it.Notes)
	clean := map[string]string{}
	for k, v := range it.Attributes {
		if v = strings.TrimSpace(v); v != "" {
			clean[k] = v
		}
	}
	it.Attributes = clean
}

// validateBase prüft die Felder, die jedes Item hat (K2).
func (it Item) validateBase() []string {
	var v validator
	v.check(it.Brand != "", "Marke fehlt")
	v.check(it.Model != "", "Modell fehlt")
	v.check(it.Condition.Valid(), "Zustand ist ungültig")
	return v.problems
}
