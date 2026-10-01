package sources

import (
	"context"
	"fmt"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
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
	if aare.HydroKey != "2135" {
		t.Errorf("HydroKey = %q", aare.HydroKey)
	}
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

func TestParseHydro(t *testing.T) {
	got, err := parseHydro(fixture(t, "hydro.geojson"))
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 5 {
		t.Fatalf("got %d stations, want 5", len(got))
	}

	aare := got["2135"]
	if aare.Discharge == nil || *aare.Discharge != 67 || aare.WaterLevel == nil || *aare.WaterLevel != 501.82 {
		t.Errorf("unexpected Aare reading: %+v", aare)
	}
	if aare.DangerLevel == nil || *aare.DangerLevel != 1 {
		t.Errorf("Aare danger level = %v", aare.DangerLevel)
	}
	if want := time.Date(2026, 9, 30, 13, 20, 0, 0, time.UTC); aare.MeasuredAt == nil || !aare.MeasuredAt.Equal(want) {
		t.Errorf("MeasuredAt = %v, want %v", aare.MeasuredAt, want)
	}

	// A lake gauge has a level but no discharge.
	if lake := got["2208"]; lake.Discharge != nil || lake.WaterLevel == nil || *lake.WaterLevel != 429.13 {
		t.Errorf("unexpected lake reading: %+v", lake)
	}
	// l/s is converted to m³/s.
	if melera := got["2206"]; melera.Discharge == nil || !near(*melera.Discharge, 0.019) || *melera.DangerLevel != 3 {
		t.Errorf("unexpected l/s reading: %+v", melera)
	}
	// A relative level ("0.05 m") is dropped, the discharge kept.
	if brook := got["2282"]; brook.WaterLevel != nil || brook.Discharge == nil {
		t.Errorf("relative level should be dropped: %+v", brook)
	}
	// Gauges without danger levels have none.
	if canal := got["2446"]; canal.DangerLevel != nil {
		t.Errorf("not_applicable should have no danger level: %v", *canal.DangerLevel)
	}
}

func TestParseHydroErrors(t *testing.T) {
	for _, body := range []string{`not json`, `{"features": []}`, `{"features": [{"properties": {"key": "1"}}]}`} {
		if _, err := parseHydro([]byte(body)); err == nil {
			t.Errorf("parseHydro(%q) returned no error", body)
		}
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
	if zh.HydroKey != "2209" || untersee.HydroKey != "2043" || stations[2].HydroKey != "" {
		t.Errorf("lake gauges = %q %q %q", zh.HydroKey, untersee.HydroKey, stations[2].HydroKey)
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

func TestParseZurich(t *testing.T) {
	s, err := parseZurich(fixture(t, "zurich.json"))
	if err != nil {
		t.Fatal(err)
	}
	// Three rows, one flagged: two points, oldest first; the newest is current.
	if len(s.Recent) != 2 || !s.Recent[0].Time.Before(s.Recent[1].Time) {
		t.Fatalf("Recent = %+v", s.Recent)
	}
	if want := time.Date(2026, 10, 1, 13, 30, 0, 0, time.UTC); s.Temperature != 22 || !s.MeasuredAt.Equal(want) {
		t.Errorf("current = %v at %v", s.Temperature, s.MeasuredAt)
	}
	if s.Kind != station.Lake || s.WaterBody != "Zürichsee" || s.HydroKey != "2209" || s.Fallback {
		t.Errorf("unexpected station: %+v", s)
	}
	if _, err := parseZurich([]byte(`{"ok":true,"result":[]}`)); err == nil {
		t.Error("expected an error without values")
	}
}

func TestParseWiewarm(t *testing.T) {
	temp, at, err := parseWiewarm(fixture(t, "wiewarm.json"), "23")
	if err != nil {
		t.Fatal(err)
	}
	if want := time.Date(2026, 10, 1, 15, 18, 0, 0, time.UTC); temp != 19.4 || !at.Equal(want) {
		t.Errorf("got %v at %v", temp, at)
	}
	// Fractional seconds in the date are tolerated.
	if _, _, err := parseWiewarm(fixture(t, "wiewarm.json"), "98"); err != nil {
		t.Error(err)
	}
	if _, _, err := parseWiewarm(fixture(t, "wiewarm.json"), "1"); err == nil {
		t.Error("expected an error for an unknown pool")
	}
}

func TestParseDatalakes(t *testing.T) {
	points, err := parseDatalakes(fixture(t, "datalakes.json"), "x", "y", "y2")
	if err != nil {
		t.Fatal(err)
	}
	// null, quality-flagged and implausible values are dropped.
	if len(points) != 2 || points[0].Value != 19.1 || points[1].Value != 19.4 {
		t.Fatalf("points = %+v", points)
	}
	// Without (usable) quality flags the values are taken as they are.
	if points, _ := parseDatalakes(fixture(t, "datalakes.json"), "x", "y", "missing"); len(points) != 3 {
		t.Errorf("without flags: %+v", points)
	}
	if _, err := parseDatalakes([]byte(`{"x":[1],"y":[null]}`), "x", "y", ""); err == nil {
		t.Error("expected an error without valid values")
	}

	h := hourly([]station.Point{
		{Time: time.Date(2026, 10, 1, 10, 0, 0, 0, time.UTC), Value: 1},
		{Time: time.Date(2026, 10, 1, 10, 50, 0, 0, time.UTC), Value: 2},
		{Time: time.Date(2026, 10, 1, 11, 10, 0, 0, time.UTC), Value: 3},
	})
	if len(h) != 2 || h[0].Value != 2 || h[1].Value != 3 {
		t.Errorf("hourly = %+v", h)
	}
}

func TestLakeGaugeIdentifiesLakesAcrossNames(t *testing.T) {
	for _, pair := range [][2]string{{"Genfersee", "Lac Léman"}, {"Luganersee", "Lago di Lugano"}, {"Neuenburgersee", "Lac de Neuchâtel"}} {
		if a, b := lakeGauge(pair[0]), lakeGauge(pair[1]); a == "" || a != b {
			t.Errorf("%s → %q, %s → %q", pair[0], a, pair[1], b)
		}
	}
}

func TestDatalakesPartialFailure(t *testing.T) {
	broken := 0 // number of datasets answering with an error
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.URL.Query().Get("datasets_id")
		if id == "" && strings.HasPrefix(r.URL.Path, "/datasets/") {
			id = strings.TrimPrefix(r.URL.Path, "/datasets/")
		}
		for _, set := range datalakesSets[:broken] {
			if id == strconv.Itoa(set.id) {
				http.Error(w, "down", http.StatusBadGateway)
				return
			}
		}
		now := time.Now().UTC()
		switch {
		case r.URL.Path == "/datasetparameters":
			set := datalakesSets[0]
			for _, s := range datalakesSets {
				if strconv.Itoa(s.id) == id {
					set = s
				}
			}
			fmt.Fprintf(w, `[{"axis":"x","parseparameter":"time"},{"axis":"y","parseparameter":%q}]`, set.param)
		case strings.HasPrefix(r.URL.Path, "/datasets/"):
			fmt.Fprintf(w, `{"maxdatetime":%q}`, now.Format(time.RFC3339))
		case r.URL.Path == "/files":
			fmt.Fprintf(w, `[{"id":1,"filetype":"json","maxdatetime":%q}]`, now.Format(time.RFC3339))
		default:
			fmt.Fprintf(w, `{"x":[%d],"y":[15.2]}`, now.Unix())
		}
	}))
	defer srv.Close()

	broken = 1
	d := &Datalakes{Client: srv.Client(), BaseURL: srv.URL, cache: map[int]*datalakesCache{}}
	stations, err := d.Fetch(context.Background())
	if err != nil {
		t.Errorf("one broken dataset must not fail the source: %v", err)
	}
	if len(stations) != len(datalakesSets)-1 {
		t.Errorf("got %d stations, want %d", len(stations), len(datalakesSets)-1)
	}

	broken = len(datalakesSets)
	d = &Datalakes{Client: srv.Client(), BaseURL: srv.URL, cache: map[int]*datalakesCache{}}
	if _, err := d.Fetch(context.Background()); err == nil {
		t.Error("expected an error when every dataset fails")
	}
}

func TestZurichPartialFailure(t *testing.T) {
	body := fixture(t, "zurich.json")
	broken := ""
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if broken == "all" || strings.HasSuffix(r.URL.Path, "/"+broken) {
			http.Error(w, "down", http.StatusBadGateway)
			return
		}
		w.Write(body)
	}))
	defer srv.Close()
	z := &Zurich{Client: srv.Client(), BaseURL: srv.URL}

	broken = zurichStations[0].key
	stations, err := z.Fetch(context.Background())
	if err != nil || len(stations) != len(zurichStations)-1 {
		t.Errorf("one broken station: got %d stations, error %v", len(stations), err)
	}
	broken = "all"
	if _, err := z.Fetch(context.Background()); err == nil {
		t.Error("expected an error when every station fails")
	}
}

func TestDatalakesFollowsReplacedFiles(t *testing.T) {
	current, listCalls := 100, 0
	now := time.Now().UTC()
	mux := http.NewServeMux()
	mux.HandleFunc("/datasetparameters", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`[{"axis":"x","parseparameter":"time"},{"axis":"y","parseparameter":"temperature"},
			{"axis":"y","parseparameter":"surfacetemp"},{"axis":"y","parseparameter":"surface_temp"}]`))
	})
	mux.HandleFunc("/datasets/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"maxdatetime":"` + now.Format(time.RFC3339) + `"}`))
	})
	mux.HandleFunc("/files", func(w http.ResponseWriter, r *http.Request) {
		listCalls++
		w.Write([]byte(`[{"id":` + strconv.Itoa(current) + `,"filetype":"json","maxdatetime":"` + now.Format(time.RFC3339) + `"}]`))
	})
	mux.HandleFunc("/files/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/files/"+strconv.Itoa(current) {
			http.NotFound(w, r)
			return
		}
		w.Write([]byte(`{"x":[` + strconv.FormatInt(now.Unix(), 10) + `],"y":[19.2]}`))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	d := NewDatalakes(srv.Client())
	d.BaseURL = srv.URL
	if st, err := d.Fetch(context.Background()); err != nil || len(st) != len(datalakesSets) {
		t.Fatalf("first fetch: %d stations, %v", len(st), err)
	}
	if listCalls != len(datalakesSets) {
		t.Errorf("file list read %d times, want once per dataset", listCalls)
	}

	// Same day, unchanged file: the cached id is used, no list.
	d.Fetch(context.Background())
	if listCalls != len(datalakesSets) {
		t.Errorf("file list re-read without need: %d", listCalls)
	}

	// Datalakes replaced the file: the old id 404s, the list is read again.
	current = 101
	st, err := d.Fetch(context.Background())
	if err != nil || len(st) != len(datalakesSets) || st[0].Temperature != 19.2 {
		t.Fatalf("after replacement: %d stations, %v", len(st), err)
	}
	if listCalls != 2*len(datalakesSets) {
		t.Errorf("file list read %d times after replacement", listCalls)
	}
}
