package store

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/999mattia/SwissWaterTemps/internal/history"
	"github.com/999mattia/SwissWaterTemps/internal/station"
)

type fakeFetcher struct {
	id       string
	stations []station.Station
	err      error
	panics   bool
}

func (f *fakeFetcher) ID() string   { return f.id }
func (f *fakeFetcher) Name() string { return f.id }
func (f *fakeFetcher) URL() string  { return "https://example.com/" + f.id }
func (f *fakeFetcher) Fetch(context.Context) ([]station.Station, error) {
	if f.panics {
		panic("boom")
	}
	return f.stations, f.err
}

func TestRefreshKeepsLastGoodDataPerSource(t *testing.T) {
	lakes := &fakeFetcher{id: "lakes", stations: []station.Station{{ID: "l1", Name: "Zürichsee"}}}
	rivers := &fakeFetcher{id: "rivers", stations: []station.Station{{ID: "r1", Name: "Aare"}}}
	s := New(nil, lakes, rivers)

	first := time.Date(2026, 7, 1, 12, 0, 0, 0, time.UTC)
	s.now = func() time.Time { return first }
	s.Refresh(context.Background())

	snap := s.Snapshot()
	if len(snap.Stations) != 2 || snap.Stations[0].Name != "Aare" {
		t.Fatalf("stations not merged and sorted: %+v", snap.Stations)
	}

	// Lakes fails on the next run: its old data stays, status reports the error.
	lakes.err = errors.New("timeout")
	lakes.stations = nil
	rivers.stations = []station.Station{{ID: "r1", Name: "Aare", Temperature: 18}}
	s.now = func() time.Time { return first.Add(10 * time.Minute) }
	s.Refresh(context.Background())

	snap = s.Snapshot()
	if len(snap.Stations) != 2 {
		t.Fatalf("failed source dropped its data: %+v", snap.Stations)
	}
	if snap.Stations[0].Temperature != 18 {
		t.Errorf("healthy source not updated: %+v", snap.Stations[0])
	}
	lakeStatus := snap.Sources[0]
	if lakeStatus.OK || lakeStatus.LastError != "timeout" || !lakeStatus.LastSuccess.Equal(first) {
		t.Errorf("unexpected lake status: %+v", lakeStatus)
	}
	if !snap.Sources[1].OK {
		t.Errorf("rivers should be OK: %+v", snap.Sources[1])
	}
}

type fakeHydro struct {
	data map[string]station.Hydro
	err  error
}

func (f *fakeHydro) Fetch(context.Context) (map[string]station.Hydro, error) { return f.data, f.err }

func TestHydroIsAttachedByKeyAndKeptOnFailure(t *testing.T) {
	rivers := &fakeFetcher{id: "rivers", stations: []station.Station{
		{ID: "r1", Name: "Aare", HydroKey: "2135"},
		{ID: "r2", Name: "Emme", HydroKey: "9999"},
		{ID: "r3", Name: "Wohlensee"},
	}}
	flow := 67.0
	hydro := &fakeHydro{data: map[string]station.Hydro{"2135": {Discharge: &flow}, "": {Discharge: &flow}}}
	s := New(nil, rivers)
	s.SetHydro(hydro)
	s.Refresh(context.Background())

	check := func() {
		t.Helper()
		st := s.Snapshot().Stations
		if st[0].Hydro == nil || *st[0].Hydro.Discharge != 67 {
			t.Errorf("Aare should have flow: %+v", st[0].Hydro)
		}
		if st[1].Hydro != nil || st[2].Hydro != nil {
			t.Errorf("stations without a matching gauge got flow: %+v %+v", st[1].Hydro, st[2].Hydro)
		}
	}
	check()

	// A failed flow/level fetch keeps the last readings; temperatures are unaffected.
	hydro.data, hydro.err = nil, errors.New("timeout")
	s.Refresh(context.Background())
	check()
	if !s.Snapshot().Sources[0].OK {
		t.Error("a flow/level failure must not mark a temperature source as failing")
	}
}

func TestFallbackLakeHiddenByFreshMeasurement(t *testing.T) {
	now := time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)
	at := func(d time.Duration) *time.Time { v := now.Add(d); return &v }
	boot24 := &fakeFetcher{id: "boot24", stations: []station.Station{
		{ID: "b-zh", Name: "Zürichsee", Kind: station.Lake, HydroKey: "2209", Fallback: true},
		{ID: "b-biel", Name: "Bielersee", Kind: station.Lake, HydroKey: "2208", Fallback: true},
	}}
	measured := &fakeFetcher{id: "wapo", stations: []station.Station{
		{ID: "m-zh", Name: "Zürichsee – Tiefenbrunnen", Kind: station.Lake, HydroKey: "2209", MeasuredAt: at(-time.Hour)},
	}}
	s := New(nil, boot24, measured)
	s.now = func() time.Time { return now }
	s.Refresh(context.Background())

	ids := func() (out []string) {
		for _, st := range s.Snapshot().Stations {
			out = append(out, st.ID)
		}
		return out
	}
	if got := ids(); len(got) != 2 || got[0] != "b-biel" || got[1] != "m-zh" {
		t.Errorf("fresh measurement should hide boot24's Zürichsee: %v", got)
	}

	// Once the measurement is old, boot24's value comes back next to it.
	measured.stations[0].MeasuredAt = at(-7 * time.Hour)
	s.Refresh(context.Background())
	if got := ids(); len(got) != 3 {
		t.Errorf("stale measurement should not hide the fallback: %v", got)
	}
}

func TestRefreshSurvivesPanickingSource(t *testing.T) {
	s := New(nil, &fakeFetcher{id: "bad", panics: true}, &fakeFetcher{id: "good", stations: []station.Station{{ID: "g"}}})
	s.Refresh(context.Background())

	snap := s.Snapshot()
	if snap.Sources[0].OK || len(snap.Stations) != 1 {
		t.Errorf("unexpected snapshot: %+v", snap)
	}
}

func TestSnapshotBeforeFirstRefresh(t *testing.T) {
	snap := New(nil, &fakeFetcher{id: "a"}).Snapshot()
	if snap.Stations == nil || len(snap.Sources) != 1 || snap.Sources[0].OK {
		t.Errorf("unexpected initial snapshot: %+v", snap)
	}
}

func TestRefreshUsesDataReturnedWithError(t *testing.T) {
	cached := &fakeFetcher{id: "cached", stations: []station.Station{{ID: "c", Name: "Cached"}}, err: errors.New("refresh failed")}
	s := New(nil, cached)
	s.Refresh(context.Background())

	snap := s.Snapshot()
	if len(snap.Stations) != 1 || snap.Sources[0].OK || snap.Sources[0].LastError != "refresh failed" {
		t.Errorf("unexpected snapshot: %+v", snap)
	}
}

func openHistory(t *testing.T, path string) *history.DB {
	t.Helper()
	db, err := history.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func TestHistoryChangeAndRestore(t *testing.T) {
	path := filepath.Join(t.TempDir(), "h.db")
	db := openHistory(t, path)
	t0 := time.Date(2026, 7, 14, 12, 0, 0, 0, time.UTC)
	at := func(d time.Duration) *time.Time { v := t0.Add(d); return &v }

	f := &fakeFetcher{id: "rivers", stations: []station.Station{{ID: "r", Name: "Aare", Temperature: 17, MeasuredAt: at(-24 * time.Hour)}}}
	s := New(db, f)
	s.now = func() time.Time { return t0.Add(-24 * time.Hour) }
	s.Refresh(context.Background())

	f.stations = []station.Station{{ID: "r", Name: "Aare", Temperature: 18.2, MeasuredAt: at(0)}}
	s.now = func() time.Time { return t0 }
	s.Refresh(context.Background())

	st, ok := s.Station("r")
	if !ok || st.Change24h == nil || *st.Change24h != 1.2 {
		t.Fatalf("Change24h = %v", st.Change24h)
	}

	// A new store on the same database starts with the saved data.
	restored := New(db, &fakeFetcher{id: "rivers"})
	snap := restored.Snapshot()
	if len(snap.Stations) != 1 || snap.Stations[0].Temperature != 18.2 || !snap.Sources[0].OK || !snap.UpdatedAt.Equal(t0) {
		t.Errorf("restored snapshot: %+v", snap)
	}
	if snap.Stations[0].Change24h == nil {
		t.Error("restored station lost its 24h change")
	}
}

func TestHydroHistoryOnlyForUsedGauges(t *testing.T) {
	db := openHistory(t, filepath.Join(t.TempDir(), "h.db"))
	now := time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)
	flow := 67.0
	f := &fakeFetcher{id: "rivers", stations: []station.Station{{ID: "r", Name: "Aare", HydroKey: "2135", MeasuredAt: &now}}}
	s := New(db, f)
	s.now = func() time.Time { return now }
	s.SetHydro(&fakeHydro{data: map[string]station.Hydro{"2135": {Discharge: &flow}, "9999": {Discharge: &flow}}})
	s.Refresh(context.Background())

	from, to := now.Add(-time.Hour), now.Add(time.Hour)
	if q, _, _ := db.HydroSeries(context.Background(), "2135", from, to); len(q) != 1 {
		t.Errorf("used gauge not recorded: %+v", q)
	}
	if q, _, _ := db.HydroSeries(context.Background(), "9999", from, to); len(q) != 0 {
		t.Errorf("unused gauge recorded: %+v", q)
	}
}
