package store

import (
	"context"
	"errors"
	"testing"
	"time"

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
	s := New(lakes, rivers)

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
	s := New(&fakeFetcher{id: "bad", panics: true}, &fakeFetcher{id: "good", stations: []station.Station{{ID: "g"}}})
	s.Refresh(context.Background())

	snap := s.Snapshot()
	if snap.Sources[0].OK || len(snap.Stations) != 1 {
		t.Errorf("unexpected snapshot: %+v", snap)
	}
}

func TestSnapshotBeforeFirstRefresh(t *testing.T) {
	snap := New(&fakeFetcher{id: "a"}).Snapshot()
	if snap.Stations == nil || len(snap.Sources) != 1 || snap.Sources[0].OK {
		t.Errorf("unexpected initial snapshot: %+v", snap)
	}
}
