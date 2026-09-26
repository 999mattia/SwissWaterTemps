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

	"github.com/999mattia/SwissWaterTemps/internal/sources"
	"github.com/999mattia/SwissWaterTemps/internal/station"
)

type sourceState struct {
	fetcher  sources.Fetcher
	status   station.Source
	stations []station.Station
}

type Store struct {
	now    func() time.Time
	states []*sourceState

	mu       sync.RWMutex
	snapshot station.Snapshot
}

func New(fetchers ...sources.Fetcher) *Store {
	s := &Store{now: time.Now}
	for _, f := range fetchers {
		s.states = append(s.states, &sourceState{
			fetcher: f,
			status:  station.Source{ID: f.ID(), Name: f.Name(), URL: f.URL()},
		})
	}
	s.snapshot = s.build()
	return s
}

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

	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.now()
	for i, st := range s.states {
		r := results[i]
		if r.err != nil {
			slog.Warn("fetching source failed", "source", st.fetcher.ID(), "error", r.err)
			st.status.OK = false
			st.status.LastError = r.err.Error()
			continue
		}
		slog.Info("fetched source", "source", st.fetcher.ID(), "stations", len(r.stations))
		st.status.OK = true
		st.status.LastError = ""
		st.status.LastSuccess = &now
		st.stations = r.stations
	}
	s.snapshot = s.build()
	s.snapshot.UpdatedAt = now
}

func (s *Store) build() station.Snapshot {
	snap := station.Snapshot{
		Sources:  make([]station.Source, 0, len(s.states)),
		Stations: []station.Station{},
	}
	for _, st := range s.states {
		snap.Sources = append(snap.Sources, st.status)
		snap.Stations = append(snap.Stations, st.stations...)
	}
	sort.SliceStable(snap.Stations, func(i, j int) bool {
		return strings.ToLower(snap.Stations[i].Name) < strings.ToLower(snap.Stations[j].Name)
	})
	return snap
}

var errPanic = errors.New("source panicked")
