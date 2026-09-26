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
