// Package history stores temperature, flow and level readings and the last
// known state of every source in SQLite, so trends and charts can be shown and
// restarts start with data.
package history

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"

	"github.com/999mattia/SwissWaterTemps/internal/station"
)

// Readings are stored in hourly buckets: the latest value within an hour wins.
const bucketSize = time.Hour

type DB struct {
	db *sql.DB
}

func Open(path string) (*DB, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	dsn := "file:" + path + "?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_pragma=synchronous(NORMAL)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	// SQLite allows one writer; a single connection avoids "database is locked".
	db.SetMaxOpenConns(1)

	if _, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS readings (
			station_id  TEXT    NOT NULL,
			bucket      INTEGER NOT NULL, -- unix seconds, start of the hour
			temperature REAL    NOT NULL,
			PRIMARY KEY (station_id, bucket)
		) WITHOUT ROWID;
		-- Flow and level per BAFU gauge (station.Hydro), keyed like the gauges
		-- rather than by station, since a lake's stations share one gauge.
		CREATE TABLE IF NOT EXISTS hydro_readings (
			gauge       TEXT    NOT NULL,
			bucket      INTEGER NOT NULL,
			discharge   REAL,
			water_level REAL,
			PRIMARY KEY (gauge, bucket)
		) WITHOUT ROWID;
		CREATE TABLE IF NOT EXISTS sources (
			id       TEXT PRIMARY KEY,
			status   TEXT NOT NULL,
			stations TEXT NOT NULL
		);`); err != nil {
		db.Close()
		return nil, fmt.Errorf("create schema: %w", err)
	}
	return &DB{db: db}, nil
}

func (h *DB) Close() error { return h.db.Close() }

func bucket(t time.Time) int64 { return t.Truncate(bucketSize).Unix() }

// Record stores the current value of every station, plus any recent values a
// source provided. fallback is used as the time of stations without one.
func (h *DB) Record(ctx context.Context, stations []station.Station, fallback time.Time) error {
	tx, err := h.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO readings (station_id, bucket, temperature) VALUES (?, ?, ?)
		ON CONFLICT (station_id, bucket) DO UPDATE SET temperature = excluded.temperature`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, s := range stations {
		for _, p := range s.Recent {
			if _, err := stmt.ExecContext(ctx, s.ID, bucket(p.Time), p.Value); err != nil {
				return err
			}
		}
		at := fallback
		if s.MeasuredAt != nil {
			at = *s.MeasuredAt
		}
		if _, err := stmt.ExecContext(ctx, s.ID, bucket(at), s.Temperature); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// RecordHydro stores the current flow and level of every gauge, in the bucket
// of its measurement time (or fallback).
func (h *DB) RecordHydro(ctx context.Context, gauges map[string]station.Hydro, fallback time.Time) error {
	tx, err := h.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO hydro_readings (gauge, bucket, discharge, water_level) VALUES (?, ?, ?, ?)
		ON CONFLICT (gauge, bucket) DO UPDATE SET discharge = excluded.discharge, water_level = excluded.water_level`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for key, g := range gauges {
		at := fallback
		if g.MeasuredAt != nil {
			at = *g.MeasuredAt
		}
		if _, err := stmt.ExecContext(ctx, key, bucket(at), g.Discharge, g.WaterLevel); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// HydroSeries returns the stored flow and level of one gauge between from and
// to, averaged per day beyond 31 days like Series. A gauge that doesn't
// measure one of them gets an empty series for it.
func (h *DB) HydroSeries(ctx context.Context, gauge string, from, to time.Time) (discharge, level []station.Point, err error) {
	query := `
		SELECT bucket, discharge, water_level FROM hydro_readings
		WHERE gauge = ? AND bucket BETWEEN ? AND ? ORDER BY bucket`
	if to.Sub(from) > 31*24*time.Hour {
		query = `
			SELECT (bucket / 86400) * 86400 + 43200 AS day, AVG(discharge), AVG(water_level)
			FROM hydro_readings WHERE gauge = ? AND bucket BETWEEN ? AND ?
			GROUP BY bucket / 86400 ORDER BY day`
	}
	rows, err := h.db.QueryContext(ctx, query, gauge, from.Unix(), to.Unix())
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()

	discharge, level = []station.Point{}, []station.Point{}
	for rows.Next() {
		var ts int64
		var q, l sql.NullFloat64
		if err := rows.Scan(&ts, &q, &l); err != nil {
			return nil, nil, err
		}
		t := time.Unix(ts, 0).UTC()
		if q.Valid {
			discharge = append(discharge, station.Point{Time: t, Value: q.Float64})
		}
		if l.Valid {
			level = append(level, station.Point{Time: t, Value: l.Float64})
		}
	}
	return discharge, level, rows.Err()
}

// Series returns the readings of one station between from and to. Ranges
// longer than 31 days are averaged per (UTC) day to keep responses small.
func (h *DB) Series(ctx context.Context, stationID string, from, to time.Time) ([]station.Point, error) {
	query := `
		SELECT bucket, temperature FROM readings
		WHERE station_id = ? AND bucket BETWEEN ? AND ? ORDER BY bucket`
	if to.Sub(from) > 31*24*time.Hour {
		query = `
			SELECT (bucket / 86400) * 86400 + 43200 AS day, AVG(temperature)
			FROM readings WHERE station_id = ? AND bucket BETWEEN ? AND ?
			GROUP BY bucket / 86400 ORDER BY day`
	}
	rows, err := h.db.QueryContext(ctx, query, stationID, from.Unix(), to.Unix())
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	points := []station.Point{}
	for rows.Next() {
		var ts int64
		var v float64
		if err := rows.Scan(&ts, &v); err != nil {
			return nil, err
		}
		points = append(points, station.Point{Time: time.Unix(ts, 0).UTC(), Value: v})
	}
	return points, rows.Err()
}

// Change24h returns, per station, the difference between its current value
// and the stored value closest to 24 hours before its measurement (within
// two hours). Stations without such a value are left out.
func (h *DB) Change24h(ctx context.Context, stations []station.Station, fallback time.Time) (map[string]float64, error) {
	if len(stations) == 0 {
		return nil, nil
	}
	target := make(map[string]int64, len(stations))
	lo, hi := int64(1<<62), int64(0)
	for _, s := range stations {
		at := fallback
		if s.MeasuredAt != nil {
			at = *s.MeasuredAt
		}
		t := at.Add(-24 * time.Hour).Unix()
		target[s.ID] = t
		lo, hi = min(lo, t), max(hi, t)
	}

	const tolerance = int64(2 * time.Hour / time.Second)
	rows, err := h.db.QueryContext(ctx,
		`SELECT station_id, bucket, temperature FROM readings WHERE bucket BETWEEN ? AND ?`,
		lo-tolerance, hi+tolerance)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	type best struct {
		dist int64
		v    float64
	}
	closest := map[string]best{}
	for rows.Next() {
		var id string
		var b int64
		var v float64
		if err := rows.Scan(&id, &b, &v); err != nil {
			return nil, err
		}
		t, ok := target[id]
		if !ok {
			continue
		}
		// Compare against the middle of the bucket.
		d := abs(b + int64(bucketSize/time.Second)/2 - t)
		if c, seen := closest[id]; d <= tolerance && (!seen || d < c.dist) {
			closest[id] = best{d, v}
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	out := make(map[string]float64, len(closest))
	for _, s := range stations {
		if c, ok := closest[s.ID]; ok {
			out[s.ID] = math.Round((s.Temperature-c.v)*10) / 10
		}
	}
	return out, nil
}

// Prune deletes readings (temperature, flow and level) older than the given time.
func (h *DB) Prune(ctx context.Context, before time.Time) (int64, error) {
	var total int64
	for _, table := range []string{"readings", "hydro_readings"} {
		res, err := h.db.ExecContext(ctx, `DELETE FROM `+table+` WHERE bucket < ?`, before.Unix())
		if err != nil {
			return total, err
		}
		n, _ := res.RowsAffected()
		total += n
	}
	return total, nil
}

// SourceState is what is persisted per source to survive restarts.
type SourceState struct {
	Status   station.Source
	Stations []station.Station
}

func (h *DB) SaveSource(ctx context.Context, state SourceState) error {
	status, err := json.Marshal(state.Status)
	if err != nil {
		return err
	}
	stations, err := json.Marshal(state.Stations)
	if err != nil {
		return err
	}
	_, err = h.db.ExecContext(ctx, `
		INSERT INTO sources (id, status, stations) VALUES (?, ?, ?)
		ON CONFLICT (id) DO UPDATE SET status = excluded.status, stations = excluded.stations`,
		state.Status.ID, status, stations)
	return err
}

func (h *DB) LoadSources(ctx context.Context) (map[string]SourceState, error) {
	rows, err := h.db.QueryContext(ctx, `SELECT id, status, stations FROM sources`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := map[string]SourceState{}
	for rows.Next() {
		var id, status, stations string
		if err := rows.Scan(&id, &status, &stations); err != nil {
			return nil, err
		}
		var st SourceState
		if json.Unmarshal([]byte(status), &st.Status) != nil || json.Unmarshal([]byte(stations), &st.Stations) != nil {
			continue // unreadable row from an older version; it is rewritten on the next refresh
		}
		out[id] = st
	}
	return out, rows.Err()
}

func abs(v int64) int64 {
	if v < 0 {
		return -v
	}
	return v
}
