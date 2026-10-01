// Package store polls all data sources in the background and keeps the latest
// good result of each one in memory, so requests never wait on upstream sites.
package store

import (
	"context"
	"errors"
	"log/slog"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/999mattia/SwissWaterTemps/internal/history"
	"github.com/999mattia/SwissWaterTemps/internal/sources"
	"github.com/999mattia/SwissWaterTemps/internal/station"
)

// History persists readings and source state. It is optional: without it the
// store works in memory only and stations have no 24 hour change.
type History interface {
	Record(ctx context.Context, stations []station.Station, fallback time.Time) error
	RecordHydro(ctx context.Context, gauges map[string]station.Hydro, fallback time.Time) error
	Change24h(ctx context.Context, stations []station.Station, fallback time.Time) (map[string]float64, error)
	SaveSource(ctx context.Context, state history.SourceState) error
	LoadSources(ctx context.Context) (map[string]history.SourceState, error)
}

// HydroFetcher returns flow and level readings keyed by BAFU station key.
type HydroFetcher interface {
	Fetch(ctx context.Context) (map[string]station.Hydro, error)
}

type sourceState struct {
	fetcher  sources.Fetcher
	status   station.Source
	stations []station.Station
}

type Store struct {
	now     func() time.Time
	states  []*sourceState
	history History
	// hydroFetcher is optional; hydro keeps its last good result, and a failure
	// only means stale or missing flow/level values, never missing temperatures.
	hydroFetcher HydroFetcher
	hydro        map[string]station.Hydro

	mu       sync.RWMutex
	snapshot station.Snapshot
}

// New creates a store. If h is not nil, the last saved state of every source
// is loaded so the store has data before the first refresh.
func New(h History, fetchers ...sources.Fetcher) *Store {
	s := &Store{now: time.Now, history: h}
	for _, f := range fetchers {
		s.states = append(s.states, &sourceState{
			fetcher: f,
			status:  station.Source{ID: f.ID(), Name: f.Name(), URL: f.URL()},
		})
	}

	if h != nil {
		saved, err := h.LoadSources(context.Background())
		if err != nil {
			slog.Warn("loading saved source state failed", "error", err)
		}
		var newest time.Time
		for _, st := range s.states {
			if prev, ok := saved[st.status.ID]; ok {
				st.stations = prev.Stations
				st.status.OK = prev.Status.OK
				st.status.LastError = prev.Status.LastError
				st.status.LastSuccess = prev.Status.LastSuccess
				if prev.Status.LastSuccess != nil && prev.Status.LastSuccess.After(newest) {
					newest = *prev.Status.LastSuccess
				}
			}
		}
		s.snapshot.UpdatedAt = newest
	}

	s.snapshot = s.build(s.snapshot.UpdatedAt)
	return s
}

// usedGauges keeps the gauges some station is attached to: BAFU publishes
// about 200, fewer than half of them measure water we have a temperature for,
// and only those are worth keeping history of.
func (s *Store) usedGauges(all map[string]station.Hydro) map[string]station.Hydro {
	used := map[string]station.Hydro{}
	for _, st := range s.states {
		for _, stn := range st.stations {
			if h, ok := all[stn.HydroKey]; ok && stn.HydroKey != "" {
				used[stn.HydroKey] = h
			}
		}
	}
	return used
}

// SetHydro adds flow and level readings to the stations that have a gauge.
// Call it before Run.
func (s *Store) SetHydro(f HydroFetcher) { s.hydroFetcher = f }

// Snapshot returns the most recent combined data. It never blocks on the network.
func (s *Store) Snapshot() station.Snapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.snapshot
}

// Run refreshes immediately and then on every tick until ctx is cancelled.
func (s *Store) Run(ctx context.Context, interval time.Duration) {
	s.Refresh(ctx)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.Refresh(ctx)
		}
	}
}

// Refresh fetches all sources in parallel. A failing source keeps its previous
// stations and is reported as not OK; it never affects the other sources.
func (s *Store) Refresh(ctx context.Context) {
	type result struct {
		stations []station.Station
		err      error
	}
	results := make([]result, len(s.states))

	var wg sync.WaitGroup
	var hydro map[string]station.Hydro
	if s.hydroFetcher != nil {
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					slog.Error("flow/level fetch panicked", "panic", r)
				}
			}()
			h, err := s.hydroFetcher.Fetch(ctx)
			if err != nil {
				slog.Warn("fetching flow/level failed", "error", err)
				return
			}
			hydro = h
		}()
	}
	for i, st := range s.states {
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					slog.Error("source panicked", "source", st.fetcher.ID(), "panic", r)
					results[i].err = errPanic
				}
			}()
			stations, err := st.fetcher.Fetch(ctx)
			results[i] = result{stations, err}
		}()
	}
	wg.Wait()
	if hydro != nil {
		s.hydro = hydro
	}

	now := s.now()
	var changed []*sourceState
	for i, st := range s.states {
		r := results[i]
		if r.err != nil {
			slog.Warn("fetching source failed", "source", st.fetcher.ID(), "error", r.err)
			st.status.OK = false
			st.status.LastError = r.err.Error()
		} else {
			slog.Info("fetched source", "source", st.fetcher.ID(), "stations", len(r.stations))
			st.status.OK = true
			st.status.LastError = ""
			st.status.LastSuccess = &now
		}
		// A source may return usable (e.g. cached) data along with an error.
		if len(r.stations) > 0 {
			st.stations = r.stations
			changed = append(changed, st)
		}
	}

	if s.history != nil {
		s.persist(ctx, changed, now)
		// After the stations are updated, so the gauges they use are known.
		if hydro != nil {
			if err := s.history.RecordHydro(ctx, s.usedGauges(hydro), now); err != nil {
				slog.Error("recording flow/level failed", "error", err)
			}
		}
	}

	snap := s.build(now)
	s.mu.Lock()
	s.snapshot = snap
	s.mu.Unlock()
}

// persist records readings, adds the 24 hour change to the stations and saves
// every source's state. Failures are logged; the in-memory data stays usable.
func (s *Store) persist(ctx context.Context, changed []*sourceState, now time.Time) {
	var all []station.Station
	for _, st := range changed {
		all = append(all, st.stations...)
	}
	if err := s.history.Record(ctx, all, now); err != nil {
		slog.Error("recording history failed", "error", err)
	}

	changes, err := s.history.Change24h(ctx, all, now)
	if err != nil {
		slog.Error("computing 24h change failed", "error", err)
	}
	for _, st := range changed {
		for i := range st.stations {
			st.stations[i].Change24h = nil
			if c, ok := changes[st.stations[i].ID]; ok {
				st.stations[i].Change24h = &c
			}
		}
	}

	for _, st := range s.states {
		state := history.SourceState{Status: st.status, Stations: st.stations}
		if err := s.history.SaveSource(ctx, state); err != nil {
			slog.Error("saving source state failed", "source", st.status.ID, "error", err)
		}
	}
}

// measuredFor is how recent a measured lake value must be to replace the
// fallback (boot24) value of the same lake.
const measuredFor = 6 * time.Hour

func (s *Store) build(updatedAt time.Time) station.Snapshot {
	snap := station.Snapshot{
		UpdatedAt: updatedAt,
		Sources:   make([]station.Source, 0, len(s.states)),
		Stations:  []station.Station{},
	}

	// Lakes with a fresh measured station; their fallback entries are hidden.
	measured := map[string]bool{}
	now := s.now()
	for _, st := range s.states {
		for _, stn := range st.stations {
			if !stn.Fallback && stn.Kind == station.Lake && stn.HydroKey != "" &&
				stn.MeasuredAt != nil && now.Sub(*stn.MeasuredAt) < measuredFor {
				measured[stn.HydroKey] = true
			}
		}
	}

	for _, st := range s.states {
		snap.Sources = append(snap.Sources, st.status)
		for _, stn := range st.stations {
			if stn.Fallback && measured[stn.HydroKey] {
				continue
			}
			stn.Hydro = nil
			if h, ok := s.hydro[stn.HydroKey]; ok && stn.HydroKey != "" {
				stn.Hydro = &h
			}
			snap.Stations = append(snap.Stations, stn)
		}
	}
	sort.SliceStable(snap.Stations, func(i, j int) bool {
		return strings.ToLower(snap.Stations[i].Name) < strings.ToLower(snap.Stations[j].Name)
	})
	return snap
}

// Station returns one station of the current snapshot.
func (s *Store) Station(id string) (station.Station, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, st := range s.snapshot.Stations {
		if st.ID == id {
			return st, true
		}
	}
	return station.Station{}, false
}

var errPanic = errors.New("source panicked")
