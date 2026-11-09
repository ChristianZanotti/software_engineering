// Package bike ist das Fahrrad-Plugin für das Verleih-Framework (C1, C3, C4).
// Alles Fahrrad-Wissen steht hier – der Kern kennt nur domain.Plugin.
package bike

import (
	"fmt"
	"slices"
	"strconv"

	"fahrradverleih/internal/domain"
)

// Plugin implementiert domain.Plugin.
type Plugin struct{}

var _ domain.Plugin = Plugin{}

const (
	kindCity     = "city"
	kindTrekking = "trekking"
	kindMTB      = "mtb"
	kindRoad     = "rennrad"
	kindEBike    = "ebike"
	kindCargo    = "lastenrad"
	kindKids     = "kinder"
	attrFrame    = "frameSize"
	attrWheel    = "wheelInch"
	attrGears    = "gears"
	attrRange    = "rangeKm"
	attrColor    = "color"
	maxKidsWheel = 24
)

var electric = []string{kindEBike, kindCargo}

func (Plugin) Category() string   { return "bike" }
func (Plugin) Name() string       { return "Fahrrad" }
func (Plugin) NamePlural() string { return "Fahrräder" }

// Kinds: Typen mit Starttarifen laut README C1.
func (Plugin) Kinds() []domain.Kind {
	return []domain.Kind{
		{Key: kindCity, Label: "Citybike", HourlyCents: 300, DailyCents: 1500},
		{Key: kindTrekking, Label: "Trekkingrad", HourlyCents: 400, DailyCents: 1800},
		{Key: kindMTB, Label: "Mountainbike", HourlyCents: 600, DailyCents: 2800},
		{Key: kindRoad, Label: "Rennrad", HourlyCents: 700, DailyCents: 3200},
		{Key: kindEBike, Label: "E-Bike", HourlyCents: 800, DailyCents: 3900},
		{Key: kindCargo, Label: "Lastenrad", HourlyCents: 900, DailyCents: 4500},
		{Key: kindKids, Label: "Kinderrad", HourlyCents: 200, DailyCents: 900},
	}
}

// Fields: Fahrrad-Attribute laut README C1.
func (Plugin) Fields() []domain.Field {
	return []domain.Field{
		{Key: attrFrame, Label: "Rahmengröße", Type: domain.FieldSelect, Required: true, Options: []string{"XS", "S", "M", "L", "XL"}, InTable: true},
		{Key: attrWheel, Label: "Laufradgröße", Type: domain.FieldSelect, Required: true, Unit: "Zoll",
			Options: []string{"12", "16", "20", "24", "26", "27.5", "28", "29"}, InTable: true},
		{Key: attrGears, Label: "Gänge", Type: domain.FieldNumber, Min: 1, Max: 30},
		{Key: attrRange, Label: "Akku-Reichweite", Type: domain.FieldNumber, Required: true, Min: 20, Max: 200, Unit: "km",
			OnlyKinds: electric, InTable: true},
		{Key: attrColor, Label: "Farbe", Type: domain.FieldText},
	}
}

// Extras: Zubehör laut README C3.
func (Plugin) Extras() []domain.Extra {
	return []domain.Extra{
		{Key: "helm", Label: "Helm", PriceCents: 200},
		{Key: "schloss", Label: "Schloss", PriceCents: 100},
		{Key: "korb", Label: "Korb", PriceCents: 150},
		{Key: "packtaschen", Label: "Packtaschen", PriceCents: 300},
		{Key: "kindersitz", Label: "Kindersitz", PriceCents: 400},
		{Key: "ersatzakku", Label: "Ersatz-Akku", PriceCents: 600, OnlyKinds: electric},
	}
}

// Validate prüft die fahrradspezifischen Regeln aus C4.
// Pflichtfelder, Bereiche und "nur bei E-Bike" prüft bereits die Registry generisch.
func (Plugin) Validate(it domain.Item) error {
	if it.Kind != kindKids {
		return nil
	}
	var problems []string
	if w, err := strconv.ParseFloat(it.Attributes[attrWheel], 64); err == nil && w > maxKidsWheel {
		problems = append(problems, fmt.Sprintf("Kinderrad: Laufrad höchstens %d Zoll", maxKidsWheel))
	}
	if !slices.Contains([]string{"XS", "S"}, it.Attributes[attrFrame]) {
		problems = append(problems, "Kinderrad: Rahmengröße XS oder S")
	}
	if len(problems) > 0 {
		return &domain.ValidationError{Problems: problems}
	}
	return nil
}

// Label: "Cube Kathmandu (M, 28")".
func (Plugin) Label(it domain.Item) string {
	return fmt.Sprintf("%s %s (%s, %s\")", it.Brand, it.Model, it.Attributes[attrFrame], it.Attributes[attrWheel])
}

// SampleItems liefert 20 Beispielräder für den Seed.
func SampleItems() []domain.Item {
	b := func(kind, brand, model, frame, wheel, gears, rangeKm, color string, c domain.Condition) domain.Item {
		attrs := map[string]string{attrFrame: frame, attrWheel: wheel, attrGears: gears, attrColor: color}
		if rangeKm != "" {
			attrs[attrRange] = rangeKm
		}
		return domain.Item{Category: "bike", Kind: kind, Brand: brand, Model: model, Condition: c, Attributes: attrs}
	}
	n, g, u := domain.ConditionNew, domain.ConditionGood, domain.ConditionUsed
	return []domain.Item{
		b(kindCity, "Gazelle", "Esprit C7", "M", "28", "7", "", "Schwarz", g),
		b(kindCity, "Gazelle", "Esprit C7", "L", "28", "7", "", "Weiß", g),
		b(kindCity, "Pegasus", "Solero SL", "S", "26", "7", "", "Rot", u),
		b(kindTrekking, "Cube", "Kathmandu Pro", "M", "28", "27", "", "Grau", n),
		b(kindTrekking, "Cube", "Kathmandu Pro", "L", "28", "27", "", "Grau", g),
		b(kindTrekking, "Kalkhoff", "Endeavour 5", "XL", "28", "24", "", "Blau", g),
		b(kindMTB, "Canyon", "Neuron 6", "M", "29", "12", "", "Grün", n),
		b(kindMTB, "Specialized", "Rockhopper", "S", "27.5", "9", "", "Orange", g),
		b(kindMTB, "Trek", "Marlin 7", "L", "29", "10", "", "Schwarz", u),
		b(kindRoad, "Rose", "Reveal Four", "M", "28", "22", "", "Silber", n),
		b(kindRoad, "Canyon", "Endurace 7", "L", "28", "22", "", "Rot", g),
		b(kindEBike, "Riese & Müller", "Charger4", "M", "27.5", "11", "120", "Anthrazit", n),
		b(kindEBike, "Haibike", "Trekking 6", "L", "28", "10", "90", "Weiß", g),
		b(kindEBike, "Gazelle", "Ultimate C8", "S", "28", "8", "110", "Blau", g),
		b(kindEBike, "Cube", "Reaction Hybrid", "M", "29", "12", "100", "Schwarz", u),
		b(kindCargo, "Urban Arrow", "Family", "M", "26", "5", "80", "Schwarz", g),
		b(kindCargo, "Babboe", "Big-E", "M", "26", "7", "60", "Rot", u),
		b(kindKids, "Woom", "4", "XS", "20", "8", "", "Gelb", n),
		b(kindKids, "Woom", "5", "S", "24", "8", "", "Grün", g),
		b(kindKids, "Puky", "Cyke 16", "XS", "16", "1", "", "Pink", g),
	}
}
