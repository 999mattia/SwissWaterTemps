package sources

import (
	"context"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/999mattia/SwissWaterTemps/internal/station"
)

func fixture(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func near(a, b float64) bool { return math.Abs(a-b) < 0.01 }

func TestParseBAFU(t *testing.T) {
	stations, err := parseBAFU(fixture(t, "bafu.geojson"))
	if err != nil {
		t.Fatal(err)
	}
	if len(stations) != 2 {
		t.Fatalf("got %d stations, want 2 (station without value skipped)", len(stations))
	}

	aare := stations[0]
	if aare.ID != "bafu-2135" || aare.Name != "Aare - Bern, Schönau" || aare.WaterBody != "Aare" || aare.Kind != station.River {
		t.Errorf("unexpected station: %+v", aare)
	}
	if aare.Temperature != 17.83 || *aare.Min24h != 16.9 || *aare.Max24h != 18.4 {
		t.Errorf("unexpected values: %v %v %v", aare.Temperature, *aare.Min24h, *aare.Max24h)
	}
	if want := time.Date(2026, 7, 14, 8, 40, 0, 0, time.UTC); !aare.MeasuredAt.Equal(want) {
		t.Errorf("MeasuredAt = %v, want %v", aare.MeasuredAt, want)
	}
	// LV95 coordinates near Bern are converted to WGS84.
	if !near(*aare.Lat, 46.95) || !near(*aare.Lon, 7.44) {
		t.Errorf("coordinates = %v, %v", *aare.Lat, *aare.Lon)
	}

	rhein := stations[1]
	if rhein.ID != "bafu-2091" || rhein.Temperature != 21.2 {
		t.Errorf("unexpected station: %+v", rhein)
	}
	if rhein.Min24h != nil || rhein.Max24h != nil {
		t.Errorf("missing min/max should be nil, got %v %v", rhein.Min24h, rhein.Max24h)
	}
	// Timestamps without offset are interpreted as Swiss local time.
	if want := time.Date(2026, 7, 14, 8, 40, 0, 0, time.UTC); !rhein.MeasuredAt.Equal(want) {
		t.Errorf("MeasuredAt = %v, want %v", rhein.MeasuredAt, want)
	}
	if !near(*rhein.Lat, 47.5596) || !near(*rhein.Lon, 7.5886) {
		t.Errorf("WGS84 coordinates changed: %v, %v", *rhein.Lat, *rhein.Lon)
	}
}

func TestParseBAFUErrors(t *testing.T) {
	for _, body := range []string{`not json`, `{"features": []}`} {
		if _, err := parseBAFU([]byte(body)); err == nil {
			t.Errorf("parseBAFU(%q) returned no error", body)
		}
	}
}

func TestParseBoot24(t *testing.T) {
	stations, err := parseBoot24(fixture(t, "boot24.html"))
	if err != nil {
		t.Fatal(err)
	}
	if len(stations) != 3 {
		t.Fatalf("got %d stations, want 3 (row without temperature skipped)", len(stations))
	}

	zh := stations[0]
	if zh.ID != "boot24-zurichsee" || zh.Temperature != 21.5 || zh.Kind != station.Lake {
		t.Errorf("unexpected station: %+v", zh)
	}
	if zh.MeasuredAt == nil || zh.MeasuredAt.Hour() != 8 {
		t.Errorf("MeasuredAt = %v", zh.MeasuredAt)
	}
	if zh.Lat == nil || !near(*zh.Lat, 47.25) {
		t.Errorf("Zürichsee should have coordinates, got %v", zh.Lat)
	}

	untersee := stations[1]
	if untersee.Temperature != 19.8 {
		t.Errorf("comma decimal not parsed: %v", untersee.Temperature)
	}
	if untersee.Lat == nil || !near(*untersee.Lat, 47.68) {
		t.Errorf("Untersee should match the more specific entry, got %v", untersee.Lat)
	}

	if stations[2].Lat != nil || stations[2].MeasuredAt != nil {
		t.Errorf("unknown lake should have no coordinates or date: %+v", stations[2])
	}
}

func TestParseBoot24ChangedLayout(t *testing.T) {
	if _, err := parseBoot24([]byte(`<html><body><table></table></body></html>`)); err == nil {
		t.Error("expected an error when no rows are found")
	}
}

func TestParseHikaWetter(t *testing.T) {
	stations, err := parseHikaWetter(fixture(t, "hikawetter.json"))
	if err != nil {
		t.Fatal(err)
	}
	if len(stations) != 1 || stations[0].Name != "Wohlensee" || stations[0].Temperature != 19.4 {
		t.Errorf("unexpected stations: %+v", stations)
	}
}

func TestParseHikaWetterMissingValue(t *testing.T) {
	// Previously a missing value was shown as 0 °C.
	if _, err := parseHikaWetter([]byte(`{"see":{}}`)); err == nil {
		t.Error("expected an error for a missing temperature")
	}
}

func TestFetchHTTPErrors(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("User-Agent") == "" {
			t.Error("missing User-Agent")
		}
		http.Error(w, "down", http.StatusBadGateway)
	}))
	defer srv.Close()

	fetchers := []Fetcher{
		&BAFU{Client: srv.Client(), DataURL: srv.URL},
		&Boot24{Client: srv.Client(), PageURL: srv.URL},
		&HikaWetter{Client: srv.Client(), DataURL: srv.URL},
	}
	for _, f := range fetchers {
		if _, err := f.Fetch(context.Background()); err == nil {
			t.Errorf("%s: expected an error for HTTP 502", f.ID())
		}
	}
}

func TestSlug(t *testing.T) {
	for in, want := range map[string]string{
		"Zürichsee":             "zurichsee",
		"Lac Léman (Genève)":    "lac-leman-geneve",
		"  Vierwaldstättersee ": "vierwaldstattersee",
	} {
		if got := slug(in); got != want {
			t.Errorf("slug(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestLakeCoordinatesOrder(t *testing.T) {
	for name, wantLat := range map[string]float64{
		"Bodensee Untersee": 47.68,
		"Bodensee Obersee":  47.60,
		"Zürichsee Obersee": 47.21,
		"Zürichsee":         47.25,
		"Lac Léman":         46.45,
	} {
		lat, _ := lakeCoordinates(name)
		if lat == nil || !near(*lat, wantLat) {
			t.Errorf("%s: lat = %v, want %v", name, lat, wantLat)
		}
	}
}
