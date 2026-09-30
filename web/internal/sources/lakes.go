package sources

import "strings"

type lakeCentre struct {
	key      string
	lat, lon float64
}

// lakeCentres holds approximate lake centres so lakes from sources without
// coordinates can still be placed on the map and sorted by distance. Keys are
// slugs matched as substrings of a lake's slug, and the first match wins, so
// the order matters: "Bodensee Untersee" must hit "untersee", "Bodensee
// Obersee" must hit "bodensee" and "Zürichsee Obersee" must hit "obersee".
var lakeCentres = []lakeCentre{
	{"untersee", 47.68, 9.02},
	{"bodensee", 47.60, 9.40},
	{"obersee", 47.21, 8.93},
	{"zurichsee", 47.25, 8.68},
	{"genfersee", 46.45, 6.55},
	{"lac-leman", 46.45, 6.55},
	{"leman", 46.45, 6.55},
	{"neuenburgersee", 46.90, 6.85},
	{"lac-de-neuchatel", 46.90, 6.85},
	{"vierwaldstattersee", 47.00, 8.45},
	{"thunersee", 46.69, 7.72},
	{"brienzersee", 46.73, 7.97},
	{"bielersee", 47.08, 7.17},
	{"lac-de-bienne", 47.08, 7.17},
	{"walensee", 47.12, 9.20},
	{"zugersee", 47.12, 8.48},
	{"lago-maggiore", 46.10, 8.75},
	{"langensee", 46.10, 8.75},
	{"lago-di-lugano", 45.98, 9.00},
	{"luganersee", 45.98, 9.00},
	{"ceresio", 45.98, 9.00},
	{"murtensee", 46.93, 7.08},
	{"lac-de-morat", 46.93, 7.08},
	{"sempachersee", 47.14, 8.15},
	{"hallwilersee", 47.28, 8.22},
	{"baldeggersee", 47.20, 8.26},
	{"greifensee", 47.35, 8.68},
	{"pfaffikersee", 47.35, 8.78},
	{"agerisee", 47.12, 8.62},
	{"sarnersee", 46.87, 8.21},
	{"lungerersee", 46.79, 8.16},
	{"sihlsee", 47.13, 8.78},
	{"lauerzersee", 47.03, 8.60},
	{"klontalersee", 47.03, 9.00},
	{"turlersee", 47.27, 8.50},
	{"silsersee", 46.42, 9.74},
	{"silvaplanersee", 46.45, 9.79},
	{"st-moritzersee", 46.49, 9.84},
	{"lac-de-joux", 46.63, 6.28},
	{"lac-de-la-gruyere", 46.65, 7.08},
	{"caumasee", 46.82, 9.29},
	{"oeschinensee", 46.50, 7.73},
	{"wohlensee", 46.965, 7.36},
}

// lakeGauges maps lakes to the BAFU station that measures their level, matched
// like lakeCentres (slug substrings, first match wins, so "untersee" comes
// before "bodensee"). Where BAFU has several gauges on a lake, the one nearest
// the middle is used.
var lakeGauges = []struct{ key, station string }{
	{"untersee", "2043"},
	{"bodensee", "2032"},
	{"zurichsee", "2209"},
	{"genfersee", "2027"},
	{"leman", "2027"},
	{"neuenburgersee", "2154"},
	{"neuchatel", "2154"},
	{"vierwaldstattersee", "2207"},
	{"thunersee", "2093"},
	{"brienzersee", "2023"},
	{"bielersee", "2208"},
	{"lac-de-bienne", "2208"},
	{"walensee", "2118"},
	{"zugersee", "2017"},
	{"lago-maggiore", "2022"},
	{"langensee", "2022"},
	{"luganersee", "2101"},
	{"lago-di-lugano", "2101"},
	{"ceresio", "2101"},
	{"murtensee", "2004"},
	{"lac-de-morat", "2004"},
	{"sempachersee", "2168"},
	{"hallwilersee", "2097"},
	{"baldeggersee", "2137"},
	{"agerisee", "2031"},
	{"sarnersee", "2088"},
	{"lauerzersee", "2484"},
	{"silsersee", "2072"},
	{"silvaplanersee", "2073"},
	{"st-moritzersee", "2066"},
	{"lac-de-joux", "2007"},
}

// lakeGauge returns the BAFU level station of a lake, or "" if it has none.
func lakeGauge(name string) string {
	s := slug(name)
	for _, g := range lakeGauges {
		if strings.Contains(s, g.key) {
			return g.station
		}
	}
	return ""
}

func lakeCoordinates(name string) (lat, lon *float64) {
	s := slug(name)
	for _, c := range lakeCentres {
		if strings.Contains(s, c.key) {
			return ptr(c.lat), ptr(c.lon)
		}
	}
	return nil, nil
}
