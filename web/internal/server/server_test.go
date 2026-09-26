package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/999mattia/SwissWaterTemps/internal/station"
)

func testHandler() http.Handler {
	snap := station.Snapshot{
		UpdatedAt: time.Date(2026, 7, 1, 12, 0, 0, 0, time.UTC),
		Stations: []station.Station{
			{ID: "a", Name: "Aare", Kind: station.River, Temperature: 17.5},
			{ID: "z", Name: "Zürichsee", Kind: station.Lake, Temperature: 21},
		},
	}
	static := fstest.MapFS{
		"index.html":           {Data: []byte("<html>app</html>")},
		"assets/app-abc123.js": {Data: []byte("console.log(1)")},
		"manifest.webmanifest": {Data: []byte("{}")},
	}
	return New(func() station.Snapshot { return snap }, static)
}

func do(h http.Handler, path string, header ...string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, path, nil)
	for i := 0; i+1 < len(header); i += 2 {
		req.Header.Set(header[i], header[i+1])
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestStations(t *testing.T) {
	h := testHandler()
	rec := do(h, "/api/v1/stations")
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d", rec.Code)
	}
	var snap station.Snapshot
	if err := json.Unmarshal(rec.Body.Bytes(), &snap); err != nil {
		t.Fatal(err)
	}
	if len(snap.Stations) != 2 {
		t.Errorf("got %d stations", len(snap.Stations))
	}

	etag := rec.Header().Get("ETag")
	if rec := do(h, "/api/v1/stations", "If-None-Match", etag); rec.Code != http.StatusNotModified {
		t.Errorf("conditional request: status %d, want 304", rec.Code)
	}
	if rec := do(h, "/api/v1/stations", "Accept-Encoding", "gzip"); rec.Header().Get("Content-Encoding") == "gzip" {
		// Small bodies are not worth compressing.
		t.Error("tiny response should not be gzipped")
	}
}

func TestLegacyTemperatures(t *testing.T) {
	rec := do(testHandler(), "/api/temperatures")
	var body map[string][]map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if len(body["lakeTemperatures"]) != 1 || body["lakeTemperatures"][0]["name"] != "Zürichsee" {
		t.Errorf("lakes: %+v", body["lakeTemperatures"])
	}
	if len(body["riverTemperatures"]) != 1 || body["riverTemperatures"][0]["temperature"] != 17.5 {
		t.Errorf("rivers: %+v", body["riverTemperatures"])
	}
}

func TestFrontend(t *testing.T) {
	h := testHandler()
	tests := []struct {
		path, wantBody, wantCache string
		wantCode                  int
	}{
		{"/", "app", "no-cache", 200},
		{"/some/client/route", "app", "no-cache", 200},
		{"/assets/app-abc123.js", "console", "immutable", 200},
		{"/assets/missing.js", "", "", 404},
		{"/assets", "", "", 404},
		{"/api/v2/unknown", "", "", 404},
		{"/healthz", "ok", "no-store", 200},
	}
	for _, tt := range tests {
		rec := do(h, tt.path)
		if rec.Code != tt.wantCode {
			t.Errorf("%s: status %d, want %d", tt.path, rec.Code, tt.wantCode)
			continue
		}
		if !strings.Contains(rec.Body.String(), tt.wantBody) {
			t.Errorf("%s: body %q", tt.path, rec.Body.String())
		}
		if !strings.Contains(rec.Header().Get("Cache-Control"), tt.wantCache) {
			t.Errorf("%s: Cache-Control %q", tt.path, rec.Header().Get("Cache-Control"))
		}
	}

	if ct := do(h, "/manifest.webmanifest").Header().Get("Content-Type"); ct != "application/manifest+json" {
		t.Errorf("manifest Content-Type = %q", ct)
	}
}
