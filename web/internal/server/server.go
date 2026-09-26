// Package server exposes the cached station data as JSON and serves the frontend.
package server

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"errors"
	"io/fs"
	"log/slog"
	"net/http"
	"path"
	"strconv"
	"strings"

	"github.com/999mattia/SwissWaterTemps/internal/station"
)

type SnapshotFunc func() station.Snapshot

type Server struct {
	snapshot SnapshotFunc
	static   fs.FS
}

func New(snapshot SnapshotFunc, static fs.FS) http.Handler {
	s := &Server{snapshot: snapshot, static: static}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/stations", s.stations)
	mux.HandleFunc("GET /api/temperatures", s.legacyTemperatures)
	mux.HandleFunc("GET /healthz", s.health)
	mux.HandleFunc("GET /api/", func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "not found", http.StatusNotFound)
	})
	mux.HandleFunc("GET /", s.frontend)

	return recoverer(mux)
}

func (s *Server) stations(w http.ResponseWriter, r *http.Request) {
	snap := s.snapshot()
	etag := `"` + strconv.FormatInt(snap.UpdatedAt.UnixNano(), 36) + `"`
	w.Header().Set("ETag", etag)
	w.Header().Set("Cache-Control", "public, max-age=60")
	if r.Header.Get("If-None-Match") == etag {
		w.WriteHeader(http.StatusNotModified)
		return
	}
	writeJSON(w, r, snap)
}

type legacyRecord struct {
	Name        string  `json:"name"`
	Temperature float64 `json:"temperature"`
}

// legacyTemperatures keeps the v1 response shape used by the Garmin watch app.
func (s *Server) legacyTemperatures(w http.ResponseWriter, r *http.Request) {
	lakes, rivers := []legacyRecord{}, []legacyRecord{}
	for _, st := range s.snapshot().Stations {
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
