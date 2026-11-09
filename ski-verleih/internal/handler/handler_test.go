package handler

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"skiverleih/internal/store"
)

func setup(t *testing.T) (*httptest.Server, time.Time) {
	t.Helper()
	now := time.Now()
	st := store.NewMemoryStore()
	store.Seed(st, now)
	mux := http.NewServeMux()
	New(st).Register(mux)
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

func TestA1_DashboardEndpoint(t *testing.T) {
	srv, _ := setup(t)
	code, d := call(t, "GET", srv.URL+"/api/dashboard", "")
	if code != 200 || d["availableSkis"] != 17.0 || d["rentedSkis"] != 3.0 || d["totalSkis"] != 20.0 || d["persons"] != 10.0 {
		t.Errorf("Dashboard: %d %v", code, d)
	}
}

func TestA5_RentalEndpointsAndStatusCodes(t *testing.T) {
	srv, now := setup(t)
	start := now.Add(72 * time.Hour).Format(time.RFC3339)

	code, r := call(t, "POST", srv.URL+"/api/rentals", `{"skiId":2,"personId":1,"start":"`+start+`","durationHours":26}`)
	if code != 201 || r["plannedPriceCents"] != 3900.0+2*900 || r["status"] != "reserviert" {
		t.Fatalf("Anlegen: %d %v", code, r)
	}
	code, r = call(t, "POST", srv.URL+"/api/rentals", `{"skiId":2,"personId":2,"start":"`+start+`","durationHours":1}`)
	if code != 409 || r["error"] == nil {
		t.Errorf("Doppelbuchung: %d %v", code, r)
	}
	code, _ = call(t, "POST", srv.URL+"/api/rentals", `{"skiId":2,"personId":1,"start":"`+start+`","durationHours":0}`)
	if code != 400 {
		t.Errorf("Dauer 0: %d", code)
	}
	code, _ = call(t, "GET", srv.URL+"/api/skis/999", "")
	if code != 404 {
		t.Errorf("unbekannter Ski: %d", code)
	}
}

func TestB3_QuoteEndpoint(t *testing.T) {
	srv, _ := setup(t)
	code, q := call(t, "GET", srv.URL+"/api/rentals/quote?type=alpin&hours=30", "")
	if code != 200 || q["totalCents"] != 7000.0 || q["restCapped"] != true {
		t.Errorf("Quote: %d %v", code, q)
	}
}

func TestB5_UpdatePriceValidation(t *testing.T) {
	srv, _ := setup(t)
	code, _ := call(t, "PUT", srv.URL+"/api/prices/alpin", `{"hourlyCents":1000,"dailyCents":100}`)
	if code != 400 {
		t.Errorf("ungültiger Tarif: %d", code)
	}
	code, tr := call(t, "PUT", srv.URL+"/api/prices/alpin", `{"hourlyCents":900,"dailyCents":4000}`)
	if code != 200 || tr["dailyCents"] != 4000.0 {
		t.Errorf("Tarif ändern: %d %v", code, tr)
	}
}

func TestA4_DocumentUpload(t *testing.T) {
	srv, _ := setup(t)
	upload := func(name string, data []byte) int {
		var buf bytes.Buffer
		mw := multipart.NewWriter(&buf)
		fw, _ := mw.CreateFormFile("file", name)
		fw.Write(data)
		mw.Close()
		res, err := http.Post(srv.URL+"/api/persons/1/document", mw.FormDataContentType(), &buf)
		if err != nil {
			t.Fatal(err)
		}
		res.Body.Close()
		return res.StatusCode
	}
	if code := upload("x.png", []byte("<script>alert(1)</script>")); code != 400 {
		t.Errorf("HTML-Upload: %d", code)
	}
	if code := upload("ausweis.pdf", []byte("%PDF-1.4\nhallo")); code != 200 {
		t.Errorf("PDF-Upload: %d", code)
	}
	res, err := http.Get(srv.URL + "/api/persons/1/document")
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != 200 || res.Header.Get("Content-Type") != "application/pdf" {
		t.Errorf("Download: %d %s", res.StatusCode, res.Header.Get("Content-Type"))
	}
}
