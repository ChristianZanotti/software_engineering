package domain

import (
	"net/http"
	"regexp"
	"strings"
	"time"
)

// IDDocType ist die Art des Ausweises (A4).
type IDDocType string

const (
	DocIDCard   IDDocType = "personalausweis"
	DocPassport IDDocType = "reisepass"
	DocLicense  IDDocType = "fuehrerschein"
)

func (t IDDocType) Valid() bool {
	return t == DocIDCard || t == DocPassport || t == DocLicense
}

var docNumberPattern = regexp.MustCompile(`^[A-Z0-9]{5,20}$`)

// MaxDocumentBytes ist die maximale Größe eines Ausweis-Scans (A4).
const MaxDocumentBytes = 5 << 20

var allowedDocTypes = map[string]bool{
	"image/jpeg":      true,
	"image/png":       true,
	"image/webp":      true,
	"application/pdf": true,
}

// Document ist ein hochgeladener Ausweis-Scan. Die Bytes werden nicht als JSON ausgegeben.
type Document struct {
	FileName    string    `json:"fileName"`
	ContentType string    `json:"contentType"`
	Size        int       `json:"size"`
	UploadedAt  time.Time `json:"uploadedAt"`
	Data        []byte    `json:"-"`
}

// NewDocument prüft Größe und Dateityp anhand des Inhalts (nicht der Endung) – A4.
func NewDocument(fileName string, data []byte, now time.Time) (Document, error) {
	var v validator
	v.check(len(data) > 0, "Datei ist leer")
	v.check(len(data) <= MaxDocumentBytes, "Datei ist größer als 5 MB")
	ct := http.DetectContentType(data)
	v.check(allowedDocTypes[ct], "Nur JPEG, PNG, WebP oder PDF erlaubt")
	if err := v.err(); err != nil {
		return Document{}, err
	}
	return Document{
		FileName:    strings.TrimSpace(fileName),
		ContentType: ct,
		Size:        len(data),
		UploadedAt:  now,
		Data:        data,
	}, nil
}

// Person ist ein Kunde des Verleihs (A3, A4).
type Person struct {
	ID          int       `json:"id"`
	FirstName   string    `json:"firstName"`
	LastName    string    `json:"lastName"`
	Email       string    `json:"email"`
	Phone       string    `json:"phone"`
	IDDocType   IDDocType `json:"idDocType"`
	IDDocNumber string    `json:"idDocNumber"`
	Document    *Document `json:"document,omitempty"`
	CreatedAt   time.Time `json:"createdAt"`
}

// Normalize vereinheitlicht Eingaben (Ausweisnummer in Großbuchstaben ohne Leerzeichen).
func (p *Person) Normalize() {
	p.FirstName = strings.TrimSpace(p.FirstName)
	p.LastName = strings.TrimSpace(p.LastName)
	p.Email = strings.TrimSpace(p.Email)
	p.Phone = strings.TrimSpace(p.Phone)
	p.IDDocNumber = strings.ToUpper(strings.ReplaceAll(strings.TrimSpace(p.IDDocNumber), " ", ""))
}

// Validate prüft die Akzeptanzkriterien aus A3 und A4.
func (p Person) Validate() error {
	var v validator
	v.check(p.FirstName != "", "Vorname fehlt")
	v.check(p.LastName != "", "Nachname fehlt")
	v.check(p.Email != "" || p.Phone != "", "E-Mail oder Telefon angeben")
	v.check(p.Email == "" || strings.Contains(p.Email, "@"), "E-Mail-Adresse ist ungültig")
	v.check(p.IDDocType.Valid(), "Ausweistyp fehlt oder ist ungültig")
	v.check(docNumberPattern.MatchString(p.IDDocNumber), "Ausweisnummer muss 5–20 Buchstaben/Ziffern haben")
	return v.err()
}

// FullName liefert "Vorname Nachname".
func (p Person) FullName() string {
	return p.FirstName + " " + p.LastName
}

// SameDocument meldet, ob zwei Personen denselben Ausweis angeben.
func (p Person) SameDocument(o Person) bool {
	return p.IDDocType == o.IDDocType && p.IDDocNumber == o.IDDocNumber
}
