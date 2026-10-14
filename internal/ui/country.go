package ui

import (
	"fmt"
	"strings"
)

// The ISO code alone says nothing to someone who only wants to know whether they left Brazil.
var countryNames = map[string]string{
	"AR": "Argentina", "AT": "Austria", "AU": "Australia", "BE": "Belgica",
	"BG": "Bulgaria", "BR": "Brasil", "CA": "Canada", "CH": "Suica",
	"CL": "Chile", "CN": "China", "CO": "Colombia", "CZ": "Tchequia",
	"DE": "Alemanha", "DK": "Dinamarca", "EE": "Estonia", "ES": "Espanha",
	"FI": "Finlandia", "FR": "Franca", "GB": "Reino Unido", "GR": "Grecia",
	"HK": "Hong Kong", "HU": "Hungria", "ID": "Indonesia", "IE": "Irlanda",
	"IL": "Israel", "IN": "India", "IT": "Italia", "JP": "Japao",
	"KR": "Coreia do Sul", "LT": "Lituania", "LU": "Luxemburgo", "LV": "Letonia",
	"MD": "Moldavia", "MX": "Mexico", "MY": "Malasia", "NL": "Holanda",
	"NO": "Noruega", "NZ": "Nova Zelandia", "PE": "Peru", "PH": "Filipinas",
	"PL": "Polonia", "PT": "Portugal", "PY": "Paraguai", "RO": "Romenia",
	"RS": "Servia", "RU": "Russia", "SE": "Suecia", "SG": "Singapura",
	"SK": "Eslovaquia", "TH": "Tailandia", "TR": "Turquia", "TW": "Taiwan",
	"UA": "Ucrania", "US": "Estados Unidos", "UY": "Uruguai", "VN": "Vietna",
	"ZA": "Africa do Sul",
}

// CountryLabel turns "US" into "Estados Unidos (US)", or returns the bare code when unknown.
func CountryLabel(code string) string {
	code = strings.ToUpper(code)
	if name, ok := countryNames[code]; ok {
		return fmt.Sprintf("%s (%s)", name, code)
	}
	return code
}
