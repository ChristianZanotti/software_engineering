package bike

import (
	"errors"
	"testing"

	"fahrradverleih/internal/domain"
)

func reg() *domain.Registry { return domain.NewRegistry(Plugin{}) }

func bike(kind string, attrs map[string]string) domain.Item {
	return domain.Item{Category: "bike", Kind: kind, Brand: "Cube", Model: "X", Condition: domain.ConditionGood, Attributes: attrs}
}

func TestC1_SampleItemsAreValid(t *testing.T) {
	items := SampleItems()
	if len(items) != 20 {
		t.Fatalf("%d Beispielräder, want 20", len(items))
	}
	for _, it := range items {
		if err := reg().ValidateItem(it); err != nil {
			t.Errorf("%s %s: %v", it.Brand, it.Model, err)
		}
	}
}

func TestC1_Tariffs(t *testing.T) {
	tr := reg().DefaultTariffs()
	if tr["bike/ebike"].HourlyCents != 800 || tr["bike/ebike"].DailyCents != 3900 || tr["bike/kinder"].DailyCents != 900 {
		t.Errorf("Starttarife weichen von README C1 ab: %+v", tr)
	}
	if len(tr) != 7 {
		t.Errorf("%d Typen, want 7", len(tr))
	}
}

func TestC4_EBikeNeedsRange(t *testing.T) {
	it := bike(kindEBike, map[string]string{attrFrame: "M", attrWheel: "28"})
	if err := reg().ValidateItem(it); !errors.Is(err, domain.ErrValidation) {
		t.Errorf("E-Bike ohne Reichweite muss abgelehnt werden, err=%v", err)
	}
	it.Attributes[attrRange] = "90"
	if err := reg().ValidateItem(it); err != nil {
		t.Errorf("E-Bike mit Reichweite: %v", err)
	}
}

func TestC4_RangeOnlyForElectric(t *testing.T) {
	it := bike(kindCity, map[string]string{attrFrame: "M", attrWheel: "28", attrRange: "50"})
	if err := reg().ValidateItem(it); !errors.Is(err, domain.ErrValidation) {
		t.Errorf("Citybike mit Akku-Reichweite muss abgelehnt werden, err=%v", err)
	}
}

func TestC4_KidsBikeRules(t *testing.T) {
	tooBig := bike(kindKids, map[string]string{attrFrame: "M", attrWheel: "26"})
	err := reg().ValidateItem(tooBig)
	var ve *domain.ValidationError
	if !errors.As(err, &ve) || len(ve.Problems) != 2 {
		t.Errorf("Kinderrad 26\"/M: erwartet 2 Probleme, got %v", err)
	}
	ok := bike(kindKids, map[string]string{attrFrame: "S", attrWheel: "24"})
	if err := reg().ValidateItem(ok); err != nil {
		t.Errorf("Kinderrad 24\"/S: %v", err)
	}
}

func TestC3_SpareBatteryOnlyForElectric(t *testing.T) {
	city := bike(kindCity, nil)
	if _, err := reg().ResolveExtras(city, []string{"ersatzakku"}); !errors.Is(err, domain.ErrValidation) {
		t.Errorf("Ersatz-Akku für Citybike muss abgelehnt werden, err=%v", err)
	}
	ebike := bike(kindEBike, nil)
	extras, err := reg().ResolveExtras(ebike, []string{"ersatzakku", "helm"})
	if err != nil || len(extras) != 2 || extras[0].PriceCents != 600 {
		t.Errorf("E-Bike-Zubehör: %v %v", extras, err)
	}
}

func TestC1_Label(t *testing.T) {
	it := bike(kindCity, map[string]string{attrFrame: "M", attrWheel: "28"})
	if got := (Plugin{}).Label(it); got != `Cube X (M, 28")` {
		t.Errorf("Label = %q", got)
	}
}
