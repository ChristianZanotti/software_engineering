// Package domain enthält die fachlichen Kern-Structs und deren Methoden.
// Keine I/O, kein HTTP, kein Store – nur Business-Logik.
package domain

import (
	"errors"
	"strings"
)

// Fehlerarten, die der Handler auf HTTP-Statuscodes abbildet.
var (
	ErrValidation = errors.New("ungültige Eingabe")
	ErrNotFound   = errors.New("nicht gefunden")
	ErrConflict   = errors.New("Konflikt")
)

// ValidationError sammelt mehrere Eingabefehler auf einmal (benutzerfreundlicher als nur der erste).
type ValidationError struct {
	Problems []string
}

func (e *ValidationError) Error() string { return strings.Join(e.Problems, "; ") }
func (e *ValidationError) Unwrap() error { return ErrValidation }

// validator ist ein kleiner Helfer zum Sammeln von Problemen.
type validator struct{ problems []string }

func (v *validator) check(ok bool, msg string) {
	if !ok {
		v.problems = append(v.problems, msg)
	}
}

func (v *validator) err() error {
	if len(v.problems) == 0 {
		return nil
	}
	return &ValidationError{Problems: v.problems}
}
