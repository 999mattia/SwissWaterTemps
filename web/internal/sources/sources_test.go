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
		&Alplakes{Client: srv.Client(), BaseURL: srv.URL, TTL: time.Hour, now: time.Now},
		&HikaWetter{Client: srv.Client(), DataURL: srv.URL},
	}
	for _, f := range fetchers {
		if _, err := f.Fetch(context.Background()); err == nil {
			t.Errorf("%s: expected an error for HTTP 502", f.ID())
		}
	}
}
