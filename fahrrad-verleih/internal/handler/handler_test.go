package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"fahrradverleih/internal/domain"
	"fahrradverleih/internal/plugins/bike"
	"fahrradverleih/internal/store"
)

func setup(t *testing.T) (*httptest.Server, time.Time) {
	t.Helper()
	now := time.Now()
	st := store.NewMemoryStore(domain.NewRegistry(bike.Plugin{}))
	store.Seed(st, bike.SampleItems(), now)
	mux := http.NewServeMux()
	New("Fahrrad-Verleih", st).Register(mux)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv, now
}

func call(t *testing.T, method, url, body string) (int, map[string]any) {
	t.Helper()
	req, _ := http.NewRequest(method, url, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	var out map[string]any
	_ = json.NewDecoder(res.Body).Decode(&out)
	return res.StatusCode, out
}

func TestC2_PluginsEndpoint(t *testing.T) {
	srv, _ := setup(t)
	code, m := call(t, "GET", srv.URL+"/api/plugins", "")
	plugins, _ := m["plugins"].([]any)
	if code != 200 || m["appName"] != "Fahrrad-Verleih" || len(plugins) != 1 {
		t.Fatalf("plugins: %d %v", code, m)
	}
	p := plugins[0].(map[string]any)
	if p["namePlural"] != "Fahrräder" || len(p["kinds"].([]any)) != 7 || len(p["fields"].([]any)) != 5 {
		t.Errorf("Metadaten unvollständig: %v", p)
	}
}

func TestK1_C5_DashboardEndpoint(t *testing.T) {
	srv, _ := setup(t)
	code, d := call(t, "GET", srv.URL+"/api/dashboard", "")
	if code != 200 || d["availableItems"] != 17.0 || d["rentedItems"] != 3.0 || d["totalItems"] != 20.0 || d["persons"] != 10.0 {
		t.Errorf("Dashboard: %d %v", code, d)
	}
	if len(d["byKind"].([]any)) != 7 {
		t.Errorf("byKind: %v", d["byKind"])
	}
}

func TestC6_QuoteWithExtras(t *testing.T) {
	srv, _ := setup(t)
	code, q := call(t, "GET", srv.URL+"/api/rentals/quote?category=bike&kind=ebike&hours=26&extras=helm,ersatzakku", "")
	if code != 200 || q["rentCents"] != 3900.0+1600 || q["extrasCents"] != 800.0 || q["totalCents"] != 6300.0 {
		t.Errorf("Quote: %d %v", code, q)
	}
	code, _ = call(t, "GET", srv.URL+"/api/rentals/quote?category=bike&kind=city&hours=2&extras=ersatzakku", "")
	if code != 400 {
		t.Errorf("Ersatz-Akku für Citybike: %d", code)
	}
}

func TestK5_C3_CreateRental(t *testing.T) {
	srv, now := setup(t)
	start := now.Add(96 * time.Hour).Format(time.RFC3339)
	code, r := call(t, "POST", srv.URL+"/api/rentals", `{"itemId":13,"personId":7,"start":"`+start+`","durationHours":3,"extras":["helm"]}`)
	if code != 201 || r["plannedPriceCents"] != 3*800.0+200 || r["status"] != "reserviert" {
		t.Fatalf("Anlegen: %d %v", code, r)
	}
	code, _ = call(t, "POST", srv.URL+"/api/rentals", `{"itemId":13,"personId":8,"start":"`+start+`","durationHours":1}`)
	if code != 409 {
		t.Errorf("Doppelbuchung: %d", code)
	}
}

func TestC4_CreateItemValidation(t *testing.T) {
	srv, _ := setup(t)
	code, e := call(t, "POST", srv.URL+"/api/items", `{"category":"bike","kind":"kinder","brand":"Woom","model":"6","condition":"neu","attributes":{"frameSize":"L","wheelInch":"26"}}`)
	if code != 400 || !strings.Contains(e["error"].(string), "Kinderrad") {
		t.Errorf("Kinderrad-Regel: %d %v", code, e)
	}
	code, _ = call(t, "POST", srv.URL+"/api/items", `{"category":"bike","kind":"ebike","brand":"Cube","model":"E","condition":"neu","attributes":{"frameSize":"M","wheelInch":"28","rangeKm":"80"}}`)
	if code != 201 {
		t.Errorf("E-Bike anlegen: %d", code)
	}
}
