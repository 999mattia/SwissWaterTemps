package sources

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// fakeAlplakes serves the metadata and 1D point endpoints. Temperatures rise
// by 0.1 °C per 3-hour step, starting at 15 °C at the requested start time.
type fakeAlplakes struct {
	now          time.Time
	requests     atomic.Int32
	failLake     string
	failMetadata atomic.Bool
	lastPath     atomic.Value
}

func (f *fakeAlplakes) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.requests.Add(1)
	if r.URL.Path == "/simulations/1d/metadata" {
		if f.failMetadata.Load() {
			http.Error(w, "down", http.StatusBadGateway)
			return
		}
		day := func(d time.Duration) string { return f.now.Add(d).Format(time.DateOnly) }
		fmt.Fprintf(w, `[{"model":"simstrat","lakes":[
			{"name":"zug","depth":[0,1],"start_date":"2019-01-01","end_date":%q,"missing_dates":[],"variables":{}},
			{"name":"geneva","depth":[0],"start_date":"2019-01-01","end_date":%q,"missing_dates":[],"variables":{}},
			{"name":"thun","depth":[0],"start_date":"2019-01-01","end_date":%q,"missing_dates":[],"variables":{}},
			{"name":"garda","depth":[0],"start_date":"2019-01-01","end_date":%q,"missing_dates":[],"variables":{}}
		]},{"model":"other","lakes":[]}]`, day(120*time.Hour), day(120*time.Hour), day(-30*24*time.Hour), day(120*time.Hour))
		return
	}

	// /simulations/1d/point/simstrat/{lake}/{start}/{end}/{depth}
	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/"), "/")
	if len(parts) != 8 || parts[3] != "simstrat" {
		http.NotFound(w, r)
		return
	}
	f.lastPath.Store(r.URL.RequestURI())
	lake := parts[4]
	if lake == f.failLake {
		http.Error(w, "boom", http.StatusInternalServerError)
		return
	}
	start, _ := time.Parse("200601021504", parts[5])
	end, _ := time.Parse("200601021504", parts[6])

	var times []string
	var temps []any
	for t, i := start, 0; !t.After(end); t, i = t.Add(3*time.Hour), i+1 {
		times = append(times, t.Format("2006-01-02T15:04:05+00:00"))
		if i == 5 {
			temps = append(temps, nil) // missing value
		} else {
			temps = append(temps, 15+float64(i)*0.1)
		}
	}
	json.NewEncoder(w).Encode(map[string]any{
		"time":      times,
		"depth":     map[string]any{"data": 0, "unit": "m"},
		"resample":  nil,
		"variables": map[string]any{"T": map[string]any{"data": temps, "unit": "degC"}},
	})
}

func newTestAlplakes(t *testing.T, fake *fakeAlplakes) *Alplakes {
	t.Helper()
	srv := httptest.NewServer(fake)
	t.Cleanup(srv.Close)
	a := NewAlplakes(srv.Client())
	a.BaseURL = srv.URL
	a.now = func() time.Time { return fake.now }
	return a
}

func TestAlplakesFetch(t *testing.T) {
	fake := &fakeAlplakes{now: time.Date(2026, 7, 14, 13, 30, 0, 0, time.UTC)}
	a := newTestAlplakes(t, fake)

	stations, err := a.Fetch(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	// thun's simulation ended long ago and garda is not a lake we know.
	if len(stations) != 2 {
		t.Fatalf("got %d stations, want 2: %+v", len(stations), stations)
	}

	var zug = stations[0]
	if zug.ID != "alplakes-zug" {
		zug = stations[1]
	}
	if zug.Name != "Zugersee" || !zug.Modelled || zug.Source != "alplakes" || zug.Lat == nil {
		t.Errorf("unexpected station: %+v", zug)
	}

	// The series starts 7 days (56 steps) before now at 13:30, i.e. at 13:30
	// UTC; the last step at or before now is the one at 13:30.
	want := time.Date(2026, 7, 14, 13, 30, 0, 0, time.UTC)
	if !zug.MeasuredAt.Equal(want) {
		t.Errorf("MeasuredAt = %v, want %v", zug.MeasuredAt, want)
	}
	if zug.Temperature != 20.6 { // 15 + 56 * 0.1
		t.Errorf("Temperature = %v, want 20.6", zug.Temperature)
	}
	// The 24 hour window excludes the value exactly 24 hours earlier.
	if *zug.Min24h != 19.9 || *zug.Max24h != 20.6 {
		t.Errorf("24h range = %v–%v, want 19.9–20.6", *zug.Min24h, *zug.Max24h)
	}
	if len(zug.Forecast) != 40 || !zug.Forecast[0].Time.After(want) { // 5 days of 3-hour steps
		t.Errorf("forecast has %d points starting %v", len(zug.Forecast), zug.Forecast[0].Time)
	}
	// 57 steps up to now, one of them missing.
	if len(zug.Recent) != 56 {
		t.Errorf("recent has %d points, want 56", len(zug.Recent))
	}

	path, _ := fake.lastPath.Load().(string)
	if !strings.HasSuffix(path, "/0?variables=T") || !strings.Contains(path, "/202607071330/") {
		t.Errorf("unexpected request %q", path)
	}
}

func TestAlplakesCachesSeries(t *testing.T) {
	fake := &fakeAlplakes{now: time.Date(2026, 7, 14, 13, 30, 0, 0, time.UTC)}
	a := newTestAlplakes(t, fake)
	a.TTL = 6 * time.Hour

	if _, err := a.Fetch(context.Background()); err != nil {
		t.Fatal(err)
	}
	first := fake.requests.Load()

	// Within the TTL nothing is requested, but the current value moves with time.
	fake.now = fake.now.Add(3 * time.Hour)
	stations, err := a.Fetch(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if fake.requests.Load() != first {
		t.Errorf("fetched again within the TTL")
	}
	if stations[0].Temperature != 20.7 {
		t.Errorf("current value did not advance: %v", stations[0].Temperature)
	}

	// After the TTL a failing refresh still returns the cached data, with the error.
	fake.now = fake.now.Add(4 * time.Hour)
	fake.failMetadata.Store(true)
	stations, err = a.Fetch(context.Background())
	if err == nil || len(stations) != 2 {
		t.Errorf("want cached stations and an error, got %d stations, err %v", len(stations), err)
	}
}

func TestAlplakesPartialFailure(t *testing.T) {
	fake := &fakeAlplakes{now: time.Date(2026, 7, 14, 13, 30, 0, 0, time.UTC), failLake: "geneva"}
	a := newTestAlplakes(t, fake)

	stations, err := a.Fetch(context.Background())
	if err == nil || !strings.Contains(err.Error(), "1 of 2 lakes failed") {
		t.Errorf("unexpected error: %v", err)
	}
	if len(stations) != 1 || stations[0].ID != "alplakes-zug" {
		t.Errorf("want only zug, got %+v", stations)
	}

	// Once geneva recovers it is back; if it fails again its old series is kept.
	fake.failLake = ""
	fake.now = fake.now.Add(2 * time.Hour)
	if stations, _ = a.Fetch(context.Background()); len(stations) != 2 {
		t.Fatalf("got %d stations after recovery", len(stations))
	}
	fake.failLake = "geneva"
	fake.now = fake.now.Add(2 * time.Hour)
	if stations, _ = a.Fetch(context.Background()); len(stations) != 2 {
		t.Errorf("geneva's cached series was dropped: %d stations", len(stations))
	}
}

func TestParseAlplakesPointMismatch(t *testing.T) {
	body := `{"time":["2026-07-14T00:00:00+00:00"],"variables":{"T":{"data":[1,2]}}}`
	if _, err := parseAlplakesPoint([]byte(body)); err == nil {
		t.Error("expected an error for mismatched lengths")
	}
}
