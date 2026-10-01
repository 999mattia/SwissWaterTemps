// Package server exposes the cached station data as JSON and serves the frontend.
package server

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"io/fs"
	"log/slog"
	"net/http"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/999mattia/SwissWaterTemps/internal/station"
)

// Data is the current station data, see store.Store.
type Data interface {
	Snapshot() station.Snapshot
	Station(id string) (station.Station, bool)
}

// History is the stored readings, see history.DB.
type History interface {
	// Series returns a station's temperatures between from and to.
	Series(ctx context.Context, stationID string, from, to time.Time) ([]station.Point, error)
	// HydroSeries returns a BAFU gauge's flow and level between from and to.
	HydroSeries(ctx context.Context, gauge string, from, to time.Time) (discharge, level []station.Point, err error)
}

type Server struct {
	data    Data
	history History
	static  fs.FS
}

// New builds the HTTP handler. history may be nil when no history is stored.
func New(data Data, history History, static fs.FS) http.Handler {
	s := &Server{data: data, history: history, static: static}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/stations", s.stations)
	mux.HandleFunc("GET /api/v1/stations/{id}/history", s.stationHistory)
	mux.HandleFunc("GET /api/temperatures", s.legacyTemperatures)
	mux.HandleFunc("GET /healthz", s.health)
	mux.HandleFunc("GET /api/", func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "not found", http.StatusNotFound)
	})
	mux.HandleFunc("GET /", s.frontend)

	return recoverer(mux)
}

func (s *Server) stations(w http.ResponseWriter, r *http.Request) {
	snap := s.data.Snapshot()
	etag := `"` + strconv.FormatInt(snap.UpdatedAt.UnixNano(), 36) + `"`
	w.Header().Set("ETag", etag)
	w.Header().Set("Cache-Control", "public, max-age=60")
	if r.Header.Get("If-None-Match") == etag {
		w.WriteHeader(http.StatusNotModified)
		return
	}
	writeJSON(w, r, snap)
}

type historyResponse struct {
	Station station.Station `json:"station"`
	// Temperature.
	History []station.Point `json:"history"`
	// Flow (m³/s) and level (m a.s.l.) at the station's BAFU gauge, if it has one.
	Discharge  []station.Point `json:"discharge"`
	WaterLevel []station.Point `json:"waterLevel"`
}

// stationHistory returns a station with its stored readings of the last
// ?days= days (default 7, at most 730): temperature, and flow and level.
func (s *Server) stationHistory(w http.ResponseWriter, r *http.Request) {
	st, ok := s.data.Station(r.PathValue("id"))
	if !ok {
		http.Error(w, "station not found", http.StatusNotFound)
		return
	}

	days := 7
	if v := r.URL.Query().Get("days"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 {
			http.Error(w, "invalid days", http.StatusBadRequest)
			return
		}
		days = min(n, 730)
	}

	res := historyResponse{Station: st, History: []station.Point{}, Discharge: []station.Point{}, WaterLevel: []station.Point{}}
	if s.history != nil {
		now := time.Now()
		from := now.Add(-time.Duration(days) * 24 * time.Hour)
		points, err := s.history.Series(r.Context(), st.ID, from, now)
		if err == nil && st.HydroKey != "" {
			res.Discharge, res.WaterLevel, err = s.history.HydroSeries(r.Context(), st.HydroKey, from, now)
		}
		if err != nil {
			slog.Error("reading history", "station", st.ID, "error", err)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		res.History = points
	}

	w.Header().Set("Cache-Control", "public, max-age=300")
	writeJSON(w, r, res)
}

type legacyRecord struct {
	Name        string  `json:"name"`
	Temperature float64 `json:"temperature"`
}

// legacyTemperatures keeps the v1 response shape used by the Garmin watch app.
func (s *Server) legacyTemperatures(w http.ResponseWriter, r *http.Request) {
	lakes, rivers := []legacyRecord{}, []legacyRecord{}
	for _, st := range s.data.Snapshot().Stations {
		rec := legacyRecord{Name: st.Name, Temperature: st.Temperature}
		if st.Kind == station.Lake {
			lakes = append(lakes, rec)
		} else {
			rivers = append(rivers, rec)
		}
	}
	w.Header().Set("Cache-Control", "public, max-age=60")
	writeJSON(w, r, map[string]any{"lakeTemperatures": lakes, "riverTemperatures": rivers})
}

func (s *Server) health(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, r, map[string]string{"status": "ok"})
}

// frontend serves the built single-page app. Hashed files under /assets are
// cached forever; everything else (index.html, sw.js, manifest) is revalidated
// so new deployments are picked up immediately.
func (s *Server) frontend(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimPrefix(path.Clean(r.URL.Path), "/")
	if name == "" {
		name = "index.html"
	}

	info, err := fs.Stat(s.static, name)
	switch {
	case err == nil && info.IsDir():
		// No directory listings.
		http.NotFound(w, r)
		return
	case err != nil && (!errors.Is(err, fs.ErrNotExist) || path.Ext(name) != ""):
		http.NotFound(w, r)
		return
	case err != nil:
		// Unknown extensionless paths fall back to the app shell.
		name = "index.html"
	}

	if strings.HasPrefix(name, "assets/") {
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	} else {
		w.Header().Set("Cache-Control", "no-cache")
	}
	if name == "manifest.webmanifest" {
		w.Header().Set("Content-Type", "application/manifest+json")
	}
	http.ServeFileFS(w, r, s.static, name)
}

func writeJSON(w http.ResponseWriter, r *http.Request, v any) {
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(v); err != nil {
		slog.Error("encoding JSON response", "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Add("Vary", "Accept-Encoding")
	if buf.Len() > 1024 && strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
		w.Header().Set("Content-Encoding", "gzip")
		gz := gzip.NewWriter(w)
		defer gz.Close()
		_, _ = buf.WriteTo(gz)
		return
	}
	_, _ = buf.WriteTo(w)
}

func recoverer(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				if rec == http.ErrAbortHandler {
					panic(rec)
				}
				slog.Error("handler panicked", "path", r.URL.Path, "panic", rec)
				http.Error(w, "internal error", http.StatusInternalServerError)
			}
		}()
		next.ServeHTTP(w, r)
	})
}
