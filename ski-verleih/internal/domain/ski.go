package domain

import "strings"

// SkiType ist die Ski-Kategorie; sie bestimmt auch den Tarif (B2).
type SkiType string

const (
	SkiAlpin     SkiType = "alpin"
	SkiCarving   SkiType = "carving"
	SkiFreeride  SkiType = "freeride"
	SkiLanglauf  SkiType = "langlauf"
	SkiTouren    SkiType = "touren"
	SkiSnowboard SkiType = "snowboard"
)

// SkiTypes listet alle gültigen Typen in fester Reihenfolge.
var SkiTypes = []SkiType{SkiAlpin, SkiCarving, SkiFreeride, SkiLanglauf, SkiTouren, SkiSnowboard}

func (t SkiType) Valid() bool {
	for _, k := range SkiTypes {
		if k == t {
			return true
		}
	}
	return false
}

// Condition beschreibt den Zustand eines Ski.
type Condition string

const (
	ConditionNew  Condition = "neu"
	ConditionGood Condition = "gut"
	ConditionUsed Condition = "gebraucht"
)

func (c Condition) Valid() bool {
	return c == ConditionNew || c == ConditionGood || c == ConditionUsed
}

// Ski ist ein verleihbares Paar Ski bzw. ein Snowboard (A2).
type Ski struct {
	ID        int       `json:"id"`
	Brand     string    `json:"brand"`
	Model     string    `json:"model"`
	Type      SkiType   `json:"type"`
	LengthCm  int       `json:"lengthCm"`
	Condition Condition `json:"condition"`
	Notes     string    `json:"notes"`
}

// Normalize entfernt überflüssige Leerzeichen.
func (s *Ski) Normalize() {
	s.Brand = strings.TrimSpace(s.Brand)
	s.Model = strings.TrimSpace(s.Model)
	s.Notes = strings.TrimSpace(s.Notes)
}

// Validate prüft die Akzeptanzkriterien aus A2.
func (s Ski) Validate() error {
	var v validator
	v.check(s.Brand != "", "Marke fehlt")
	v.check(s.Model != "", "Modell fehlt")
	v.check(s.Type.Valid(), "Ski-Typ ist ungültig")
	v.check(s.LengthCm >= 80 && s.LengthCm <= 220, "Länge muss zwischen 80 und 220 cm liegen")
	v.check(s.Condition.Valid(), "Zustand ist ungültig")
	return v.err()
}

// Label liefert eine kurze Anzeige wie "Atomic Redster S9".
func (s Ski) Label() string {
	return s.Brand + " " + s.Model
}
