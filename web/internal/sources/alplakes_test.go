package sources

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/999mattia/SwissWaterTemps/internal/station"
)

func TestParseAlplakesPoint(t *testing.T) {
	points, err := parseAlplakesPoint(fixture(t, "alplakes_point.json"))
	if err != nil {
		t.Fatal(err)
	}
	if len(points) != 9 {
		t.Fatalf("got %d points, want 9", len(points))
	}
	last := points[len(points)-1]
	if last.Value != 16.902 || !last.Time.Equal(time.Date(2026, 10, 13, 13, 0, 2, 880_000_000, time.UTC)) {
		t.Errorf("last point = %+v", last)
	}

	if _, err := parseAlplakesPoint([]byte(`{"time":["2026-10-13T13:00:00Z"],"variables":{"T":{"data":[]}}}`)); err == nil {
		t.Error("expected an error when times and values don't match")
	}
	points, _ = parseAlplakesPoint([]byte(`{"time":["2026-10-13T13:00:00Z","2026-10-13T16:00:00Z"],"variables":{"T":{"data":[null,17.1]}}}`))
	if len(points) != 1 {
		t.Errorf("missing values should be skipped: %+v", points)
	}
}

// alplakesServer serves metadata listing thun, upperzurich, an unknown lake and
// a lake whose simulation ended, and the Thunersee fixture series for every
// lake. status, if set, is returned for the series instead.
func alplakesServer(t *testing.T, status *atomic.Int32, seriesCalls *atomic.Int32) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/simulations/1d/metadata":
			w.Write([]byte(`[{"model":"simstrat","lakes":[
				{"name":"thun","end_date":"2026-10-18"},
				{"name":"upperzurich","end_date":"2026-10-18"},
				{"name":"atlantis","end_date":"2026-10-18"},
				{"name":"brienz","end_date":"2026-01-01"}]}]`))
		case strings.HasPrefix(r.URL.Path, "/simulations/1d/point/simstrat/"):
			seriesCalls.Add(1)
			if code := status.Load(); code != 0 {
				http.Error(w, "no", int(code))
				return
			}
			w.Write(fixture(t, "alplakes_point.json"))
		default:
			http.NotFound(w, r)
		}
	}))
}

func TestAlplakesFetch(t *testing.T) {
	var status, calls atomic.Int32
	srv := alplakesServer(t, &status, &calls)
	defer srv.Close()

	now := time.Date(2026, 10, 13, 12, 0, 0, 0, time.UTC)
	a := NewAlplakes(srv.Client())
	a.BaseURL, a.Pace = srv.URL, 0
	a.now = func() time.Time { return now }
	fetch := func() ([]station.Station, error) {
		t.Helper()
		got, err := a.Fetch(context.Background())
		a.wg.Wait()
		return got, err
	}

	// The first call only starts the refresh; it reports neither data nor error.
	if got, err := fetch(); got != nil || err != nil {
		t.Fatalf("first fetch = %d stations, %v", len(got), err)
	}
	got, err := fetch()
	if err != nil {
		t.Fatal(err)
	}
	// The unknown lake and the ended simulation are left out.
	if len(got) != 2 || calls.Load() != 2 {
		t.Fatalf("got %d stations from %d requests, want 2 and 2", len(got), calls.Load())
	}
	m := byID(got)
	thun := m["alplakes-thun"]
	// The latest value at or before now (10:00 UTC), not the later one.
	if thun.Temperature != 16.9 || !thun.MeasuredAt.Equal(time.Date(2026, 10, 13, 10, 0, 2, 880_000_000, time.UTC)) {
		t.Errorf("current value = %v at %v", thun.Temperature, thun.MeasuredAt)
	}
	if thun.Name != "Thunersee" || thun.WaterBody != "Thunersee" || thun.Kind != station.Lake || thun.HydroKey != "2093" || thun.Basin != "" {
		t.Errorf("unexpected station: %+v", thun)
	}
	if !thun.Fallback || !thun.Modelled || thun.ModelError == nil || *thun.ModelError != 0.9 {
		t.Errorf("model flags = %v %v %v", thun.Fallback, thun.Modelled, thun.ModelError)
	}
	if *thun.Min24h != 16.8 || *thun.Max24h != 17.2 || len(thun.Recent) != 8 {
		t.Errorf("min/max = %v/%v, %d recent points", *thun.Min24h, *thun.Max24h, len(thun.Recent))
	}
	if z := m["alplakes-upperzurich"]; z.Basin != "upperzurich" || z.HydroKey != "2209" {
		t.Errorf("basin = %q, gauge = %q", z.Basin, z.HydroKey)
	}

	// Within the TTL nothing is requested again.
	now = now.Add(time.Hour)
	if _, err := fetch(); err != nil || calls.Load() != 2 {
		t.Errorf("refetched within the TTL: %d requests, %v", calls.Load(), err)
	}

	// When every lake fails, the cached series stay and the source reports it.
	now = now.Add(a.TTL)
	status.Store(http.StatusBadGateway)
	fetch()
	got, err = fetch()
	if err == nil || len(got) != 2 {
		t.Errorf("got %d stations, %v; want the cached ones with an error", len(got), err)
	}

	// Two days later the cached values are too old to show.
	now = now.Add(48 * time.Hour)
	fetch()
	if got, err := fetch(); err == nil || len(got) != 0 {
		t.Errorf("stale series still shown: %d stations, %v", len(got), err)
	}
}

func TestAlplakesRetriesRateLimit(t *testing.T) {
	var status, calls atomic.Int32
	srv := alplakesServer(t, &status, &calls)
	defer srv.Close()

	a := NewAlplakes(srv.Client())
	a.BaseURL, a.Pace, a.Backoff = srv.URL, 0, 50*time.Millisecond
	a.now = func() time.Time { return time.Date(2026, 10, 13, 12, 0, 0, 0, time.UTC) }
	status.Store(http.StatusTooManyRequests)
	go func() {
		// Lift the limit while the first lake waits for its retry.
		for calls.Load() == 0 {
			time.Sleep(10 * time.Millisecond)
		}
		status.Store(0)
	}()
	a.Fetch(context.Background())
	a.wg.Wait()
	got, err := a.Fetch(context.Background())
	if err != nil || len(got) != 2 || calls.Load() != 3 {
		t.Errorf("got %d stations after %d requests, %v; want 2 after 3", len(got), calls.Load(), err)
	}
}

func TestAlplakesLakeTable(t *testing.T) {
	if len(alplakesLakes) != 131 {
		t.Errorf("%d lakes, want 131", len(alplakesLakes))
	}
	for key, l := range alplakesLakes {
		if l.name == "" || l.lat < 44 || l.lat > 54 || l.lon < 5 || l.lon > 15 {
			t.Errorf("%s: %+v", key, l)
		}
	}
	// Basins on a shared gauge must be marked, or a measurement in one hides all.
	gauges := map[string][]string{}
	for key, l := range alplakesLakes {
		if g := lakeGauge(l.name); g != "" && !l.basin {
			gauges[g] = append(gauges[g], key)
		}
	}
	for g, keys := range gauges {
		if len(keys) > 1 {
			t.Errorf("gauge %s is shared by %v without basins", g, keys)
		}
	}
}
