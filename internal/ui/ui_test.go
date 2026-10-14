package ui

import (
	"reflect"
	"testing"
)

func TestWrap(t *testing.T) {
	got := wrap("o tunel ligou, mas nada passou por ele", 16)
	want := []string{"o tunel ligou,", "mas nada passou", "por ele"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("wrap = %q, want %q", got, want)
	}

	if got := wrap("", 10); !reflect.DeepEqual(got, []string{""}) {
		t.Errorf("wrap(\"\") = %q, want one empty line", got)
	}
}

func TestCountryLabel(t *testing.T) {
	if got := CountryLabel("us"); got != "Estados Unidos (US)" {
		t.Errorf("CountryLabel(us) = %q", got)
	}
	if got := CountryLabel("XK"); got != "XK" {
		t.Errorf("CountryLabel(XK) = %q, want the bare code", got)
	}
}
