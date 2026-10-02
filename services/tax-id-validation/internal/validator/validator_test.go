package validator

import (
	"context"
	"testing"
)

func TestPatternValidators(t *testing.T) {
	ctx := context.Background()
	cases := []struct {
		v     Validator
		id    string
		valid bool
	}{
		{FirsTin, "12345678-0001", true},
		{FirsTin, "1234567-0001", false},
		{FirsTin, "12345678-001", false},
		{FirsTin, "", false},
		{KraPin, "A123456789B", true},
		{KraPin, "a123456789B", false},
		{KraPin, "A12345678B", false},
		{KraPin, "   ", false},
		{OmanVat, "OM1234567890", true},
		{OmanVat, "OM123456789", false},
		{OmanVat, "1234567890", false},
		{TraTin, "12-345678-A", true},
		{TraTin, "12-345678-a", false},
		{TraTin, "123-45678-A", false},
		{Trn{}, "123456789012345", true},
		{Trn{}, "12345678901234", false},
		{Trn{}, "1234567890123456", false},
		{Trn{}, "", false},
	}
	for _, c := range cases {
		got, err := c.v.Validate(ctx, c.id)
		if err != nil {
			t.Fatalf("%s(%q): unexpected error %v", c.v.Name(), c.id, err)
		}
		if got != c.valid {
			t.Errorf("%s(%q) = %v, want %v", c.v.Name(), c.id, got, c.valid)
		}
	}
}

func TestVatSyntax(t *testing.T) {
	cases := map[string]bool{
		"IE6388047V":     true,
		"IE 6388047V":    true,
		"ie6388047v":     true,
		"GB902194939":    true,
		"XI902194939":    true,
		"EL123456789":    true,
		"GR123456789":    false, // Greece must use the EL prefix
		"ATU12345678":    true,
		"AT12345678":     false,
		"DE123456789":    true,
		"DE12345678":     false,
		"NL123456789B01": true,
		"CY10259033P":    true,
		"CY12259033P":    false, // valvat's (?!12) exclusion
		"CY70259033P":    false, // first digit must be in [0-69]
		"CY90259033P":    true,
		"FRAB123456789":  true,
		"FROI123456789":  false, // O and I are not allowed in the key
		"xxx":            false,
		"":               false,
		"US123456789":    false,
	}
	for id, want := range cases {
		if got := VatSyntaxValid(NormalizeVat(id)); got != want {
			t.Errorf("VatSyntaxValid(%q) = %v, want %v", id, got, want)
		}
	}
}

func TestNormalizeVat(t *testing.T) {
	if got := NormalizeVat(" de-123.456 789\t"); got != "DE123456789" {
		t.Fatalf("NormalizeVat = %q", got)
	}
}
