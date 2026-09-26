package history

import (
	"context"
	"math"
	"path/filepath"
	"testing"
	"time"

	"github.com/999mattia/SwissWaterTemps/internal/station"
)

func open(t *testing.T) *DB {
	t.Helper()
	db, err := Open(filepath.Join(t.TempDir(), "sub", "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func at(t time.Time) *time.Time { return &t }

var t0 = time.Date(2026, 7, 14, 12, 0, 0, 0, time.UTC)

func TestRecordAndSeries(t *testing.T) {
	db := open(t)
	ctx := context.Background()

	// Two readings in the same hour: the later one wins.
	for i, temp := range []float64{18.0, 18.4} {
		s := station.Station{ID: "a", Temperature: temp, MeasuredAt: at(t0.Add(time.Duration(i) * 20 * time.Minute))}
		if err := db.Record(ctx, []station.Station{s}, t0); err != nil {
			t.Fatal(err)
		}
	}
	// A station without a time uses the fallback; recent values are stored too.
	lake := station.Station{ID: "b", Temperature: 21, Recent: []station.Point{
		{Time: t0.Add(-3 * time.Hour), Value: 20.5},
		{Time: t0.Add(-6 * time.Hour), Value: 20.1},
	}}
	if err := db.Record(ctx, []station.Station{lake}, t0.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}

	a, err := db.Series(ctx, "a", t0.Add(-time.Hour), t0.Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if len(a) != 1 || a[0].Value != 18.4 || !a[0].Time.Equal(t0) {
		t.Errorf("series a = %+v", a)
	}

	b, _ := db.Series(ctx, "b", t0.Add(-24*time.Hour), t0.Add(24*time.Hour))
	if len(b) != 3 || b[0].Value != 20.1 || b[2].Value != 21 {
		t.Errorf("series b = %+v", b)
	}

	if none, _ := db.Series(ctx, "unknown", t0.Add(-24*time.Hour), t0); none == nil || len(none) != 0 {
		t.Errorf("unknown station should give an empty, non-nil series: %#v", none)
	}
}

func TestSeriesDailyAverages(t *testing.T) {
	db := open(t)
	ctx := context.Background()
	start := t0.Truncate(24 * time.Hour)
	var recent []station.Point
	for h := 0; h < 48; h++ {
		recent = append(recent, station.Point{Time: start.Add(time.Duration(h) * time.Hour), Value: float64(h / 24)})
	}
	if err := db.Record(ctx, []station.Station{{ID: "a", Temperature: 5, MeasuredAt: at(start.Add(47 * time.Hour)), Recent: recent}}, start); err != nil {
		t.Fatal(err)
	}

	points, err := db.Series(ctx, "a", start.Add(-time.Hour), start.Add(60*24*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	// Hour 47 was overwritten with 5, so day two averages (23*1 + 5) / 24.
	if len(points) != 2 || points[0].Value != 0 || math.Abs(points[1].Value-28.0/24) > 1e-9 {
		t.Errorf("daily points = %+v", points)
	}
	if !points[0].Time.Equal(start.Add(12 * time.Hour)) {
		t.Errorf("daily point should be at noon UTC, got %v", points[0].Time)
	}
}

func TestChange24h(t *testing.T) {
	db := open(t)
	ctx := context.Background()

	past := []station.Station{
		{ID: "a", Temperature: 17, MeasuredAt: at(t0.Add(-24 * time.Hour))},
		{ID: "b", Temperature: 20, MeasuredAt: at(t0.Add(-30 * time.Hour))}, // too far from 24 h
	}
	if err := db.Record(ctx, past, t0); err != nil {
		t.Fatal(err)
	}

	now := []station.Station{
		{ID: "a", Temperature: 18.5, MeasuredAt: at(t0.Add(10 * time.Minute))},
		{ID: "b", Temperature: 21, MeasuredAt: at(t0)},
		{ID: "c", Temperature: 10, MeasuredAt: at(t0)},
	}
	changes, err := db.Change24h(ctx, now, t0)
	if err != nil {
		t.Fatal(err)
	}
	if len(changes) != 1 || changes["a"] != 1.5 {
		t.Errorf("changes = %v, want only a: 1.5", changes)
	}
}

func TestSourcesRoundTrip(t *testing.T) {
	db := open(t)
	ctx := context.Background()

	state := SourceState{
		Status:   station.Source{ID: "bafu", Name: "BAFU", OK: true, LastSuccess: at(t0)},
		Stations: []station.Station{{ID: "a", Name: "Aare", Temperature: 18}},
	}
	if err := db.SaveSource(ctx, state); err != nil {
		t.Fatal(err)
	}
	state.Stations[0].Temperature = 19
	if err := db.SaveSource(ctx, state); err != nil {
		t.Fatal(err)
	}

	loaded, err := db.LoadSources(ctx)
	if err != nil {
		t.Fatal(err)
	}
	got := loaded["bafu"]
	if !got.Status.OK || !got.Status.LastSuccess.Equal(t0) || len(got.Stations) != 1 || got.Stations[0].Temperature != 19 {
		t.Errorf("loaded %+v", got)
	}
}

func TestPrune(t *testing.T) {
	db := open(t)
	ctx := context.Background()
	stations := []station.Station{
		{ID: "old", Temperature: 1, MeasuredAt: at(t0.Add(-800 * 24 * time.Hour))},
		{ID: "new", Temperature: 2, MeasuredAt: at(t0)},
	}
	if err := db.Record(ctx, stations, t0); err != nil {
		t.Fatal(err)
	}
	n, err := db.Prune(ctx, t0.Add(-730*24*time.Hour))
	if err != nil || n != 1 {
		t.Errorf("pruned %d rows, err %v", n, err)
	}
}
