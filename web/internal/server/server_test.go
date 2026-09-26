package server

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/999mattia/SwissWaterTemps/internal/station"
)

type fakeData struct{ snap station.Snapshot }

func (f fakeData) Snapshot() station.Snapshot { return f.snap }
func (f fakeData) Station(id string) (station.Station, bool) {
	for _, s := range f.snap.Stations {
		if s.ID == id {
			return s, true
		}
	}
	return station.Station{}, false
}

var lastSeriesRange time.Duration

func fakeSeries(_ context.Context, id string, from, to time.Time) ([]station.Point, error) {
	lastSeriesRange = to.Sub(from)
	if id == "broken" {
		return nil, errors.New("disk on fire")
	}
	return []station.Point{{Time: from, Value: 17}}, nil
}

func testHandler() http.Handler {
	snap := station.Snapshot{
		UpdatedAt: time.Date(2026, 7, 1, 12, 0, 0, 0, time.UTC),
		Stations: []station.Station{
			{ID: "a", Name: "Aare", Kind: station.River, Temperature: 17.5},
			{ID: "z", Name: "Zürichsee", Kind: station.Lake, Temperature: 21,
				Forecast: []station.Point{{Time: time.Date(2026, 7, 2, 0, 0, 0, 0, time.UTC), Value: 21.5}}},
			{ID: "broken", Name: "Broken", Kind: station.River},
		},
	}
	static := fstest.MapFS{
		"index.html":           {Data: []byte("<html>app</html>")},
		"assets/app-abc123.js": {Data: []byte("console.log(1)")},
		"manifest.webmanifest": {Data: []byte("{}")},
	}
	return New(fakeData{snap}, fakeSeries, static)
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
	if len(snap.Stations) != 3 {
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
	if len(body["riverTemperatures"]) != 2 || body["riverTemperatures"][0]["temperature"] != 17.5 {
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

func TestStationHistory(t *testing.T) {
	h := testHandler()

	rec := do(h, "/api/v1/stations/z/history?days=30")
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body)
	}
	var body struct {
		Station  station.Station `json:"station"`
		History  []station.Point `json:"history"`
		Forecast []station.Point `json:"forecast"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Station.Name != "Zürichsee" || len(body.History) != 1 || len(body.Forecast) != 1 || body.Forecast[0].Value != 21.5 {
		t.Errorf("unexpected body: %+v", body)
	}
	if lastSeriesRange != 30*24*time.Hour {
		t.Errorf("requested range %v", lastSeriesRange)
	}

	// Stations without a forecast return an empty list, not null.
	if rec := do(h, "/api/v1/stations/a/history"); !strings.Contains(rec.Body.String(), `"forecast":[]`) {
		t.Errorf("forecast should be []: %s", rec.Body)
	}
	if do(h, "/api/v1/stations/a/history?days=5000"); lastSeriesRange != 730*24*time.Hour {
		t.Errorf("days not capped: %v", lastSeriesRange)
	}

	for path, code := range map[string]int{
		"/api/v1/stations/nope/history":        404,
		"/api/v1/stations/a/history?days=zero": 400,
		"/api/v1/stations/a/history?days=0":    400,
		"/api/v1/stations/broken/history":      500,
	} {
		if rec := do(h, path); rec.Code != code {
			t.Errorf("%s: status %d, want %d", path, rec.Code, code)
		}
	}
}

func TestStationHistoryWithoutDatabase(t *testing.T) {
	h := New(fakeData{station.Snapshot{Stations: []station.Station{{ID: "a"}}}}, nil, fstest.MapFS{})
	rec := do(h, "/api/v1/stations/a/history")
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"history":[]`) {
		t.Errorf("status %d, body %s", rec.Code, rec.Body)
	}
}
