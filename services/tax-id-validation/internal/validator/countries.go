package validator

// Country lists mirror lib/utilities/compliance/countries.rb. When that file changes,
// this one must change in the same PR — the service has no access to the Rails constants.

var countriesThatCollectTaxOnAllProducts = set(
	"IS", // Iceland
	"JP", // Japan
	"NZ", // New Zealand
	"ZA", // South Africa
	"CH", // Switzerland
	"AE", // United Arab Emirates
	"IN", // India
)

var countriesWithTaxIDProValidation = set(
	"BY", // Belarus
	"CL", // Chile
	"CO", // Colombia
	"CR", // Costa Rica
	"EC", // Ecuador
	"EG", // Egypt
	"GE", // Georgia
	"KZ", // Kazakhstan
	"MY", // Malaysia
	"MD", // Moldova
	"MA", // Morocco
	"RU", // Russia
	"SA", // Saudi Arabia
	"RS", // Serbia
	"KR", // South Korea
	"TH", // Thailand
	"TR", // Turkey
	"UA", // Ukraine
	"UZ", // Uzbekistan
	"VN", // Vietnam
)

const (
	countryAustralia = "AU"
	countrySingapore = "SG"
	countryCanada    = "CA"
	countryNorway    = "NO"
	countryBahrain   = "BH"
	countryKenya     = "KE"
	countryNigeria   = "NG"
	countryTanzania  = "TZ"
	countryOman      = "OM"
	stateQuebec      = "QC"
)

func set(codes ...string) map[string]struct{} {
	m := make(map[string]struct{}, len(codes))
	for _, c := range codes {
		m[c] = struct{}{}
	}
	return m
}

// UsesTaxIDPro reports whether a country is validated through taxid.pro rather than a
// dedicated registry or the EU VIES service.
func UsesTaxIDPro(countryCode string) bool {
	if _, ok := countriesThatCollectTaxOnAllProducts[countryCode]; ok {
		return true
	}
	_, ok := countriesWithTaxIDProValidation[countryCode]
	return ok
}
